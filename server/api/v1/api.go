package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"net/http"
	"os"
	"strings"
	"time"
	. "zo-backend/server/api"
)

type ServerApi struct {
	appToken string

	yaDiskAcc    ServerAccInfo
	dashaMailAcc ServerAccInfo
	facecastAcc  ServerAccInfo

	webinars   *map[string]ServerWebinar
	wsInfoChan chan interface{}
}

func (s *ServerApi) Init() (string, error) {
	err := godotenv.Load()
	if err != nil {
		return "", fmt.Errorf("error loading .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		return "", fmt.Errorf("$PORT must be set")
	}

	err = s.initServerApiParams()
	if err != nil {
		return "", err
	}

	s.dashaMailAcc.URI = "https://api.dashamail.com/"
	s.facecastAcc.URI = "https://facecast.net/api/"

	w := make(map[string]ServerWebinar)
	s.webinars = &w

	return port, nil
}

func (s *ServerApi) initServerApiParams() error {
	initParam := func(param *string, paramName string) error {
		*param = os.Getenv(paramName)
		if *param == "" {
			return fmt.Errorf("$%s must be set", paramName)
		}
		return nil
	}

	validateAppToken := func() error {
		const (
			VALID_APP_HASH          = "fdec6cf70915fac3acf272066f7526599ebe37f03343e4066a8957ab6055a6fd"
			INVALID_APP_TOKEN_ERROR = "invalid $APP_TOKEN"
		)

		parts := strings.Split(s.appToken, "__")
		if len(parts) < 2 {
			return fmt.Errorf(INVALID_APP_TOKEN_ERROR)
		}

		key := parts[0]
		secret := parts[1]

		sig := hmac.New(sha256.New, []byte(secret))
		sig.Write([]byte(key))

		if hex.EncodeToString(sig.Sum(nil)) != VALID_APP_HASH {
			return fmt.Errorf(INVALID_APP_TOKEN_ERROR)
		}

		return nil
	}

	if err := initParam(&s.appToken, "APP_TOKEN"); err != nil {
		return err
	} else {
		if err = validateAppToken(); err != nil {
			return err
		}
	}

	if err := initParam(&s.yaDiskAcc.ApiKey, "YANDEX_API_KEY"); err != nil {
		return err
	}

	if err := initParam(&s.dashaMailAcc.ApiKey, "DASHAMAIL_API_KEY"); err != nil {
		return err
	}

	if err := initParam(&s.facecastAcc.ApiKey, "FACECAST_API_KEY"); err != nil {
		return err
	}

	if err := initParam(&s.facecastAcc.ApiSecret, "FACECAST_API_SECRET"); err != nil {
		return err
	}

	return nil
}

// UnknownEndpoint returns a personalized JSON message.
func (s *ServerApi) UnknownEndpoint(w http.ResponseWriter, r *http.Request) {
	unknown := chi.URLParam(r, "unknown")
	err := fmt.Errorf("unknown %s API endpoint /%s", r.Method, unknown)
	SendServerResponse(w, nil, &ServerDebug{Error: err})
}

func (s *ServerApi) GetUserLK(w http.ResponseWriter, r *http.Request) {
	if email := r.URL.Query().Get("email"); email == "" {
		err := getInvalidFieldError("email", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		response, debug := s.getUserLK(email)
		SendServerResponse(w, response, debug)
	}
}

func (s *ServerApi) GetUserPoints(w http.ResponseWriter, r *http.Request) {
	if email := r.URL.Query().Get("email"); email == "" {
		err := getInvalidFieldError("email", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if pointsType := r.URL.Query().Get("type"); pointsType == "" {
		err := getInvalidFieldError("type", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		response, debug := s.getUserPoints(email, pointsType)
		SendServerResponse(w, response, debug)
	}
}

func (s *ServerApi) GetWebinarReportInfo(w http.ResponseWriter, r *http.Request) {
	if eventID := r.URL.Query().Get("eventID"); eventID == "" {
		err := getInvalidFieldError("eventID", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		response, debug := s.getWebinarReportInfo(eventID, nil)
		SendServerResponse(w, response, debug)
	}
}

func (s *ServerApi) GetCampaignsReportInfo(w http.ResponseWriter, r *http.Request) {
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	/*/
	 * Либо оба параметра - непустые строки нужного формата, либо оба - пустые строки.
	 * В последнем случае endDate - текущая дата, а startDate - дата за 30 дней до текущей даты.
	/*/
	if startDate == "" && endDate == "" {
		now := time.Now().UTC()
		now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		endDate = now.Format("2006-01-02")
		startDate = now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
	}

	if startDate == "" && endDate != "" {
		err := getInvalidFieldError("startDate", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if startDate != "" && endDate == "" {
		err := getInvalidFieldError("endDate", "string")
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		response, debug := s.getCampaignsReportInfo(startDate, endDate, nil)
		SendServerResponse(w, response, debug)
	}
}

func (s *ServerApi) FacecastLogin(w http.ResponseWriter, r *http.Request) {
	// вначале проверка актуальности данных для PERSONAL_PHRASES
	updatePersonalPhrasesDates(s.webinars)

	if body, err := ReadRequestBody(r.Body); err != nil {
		err := getBodyReadingError(err)
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if eventID, ok := body["eventID"].(string); !ok || eventID == "" {
		err := getInvalidFieldError("eventID", "string", body["eventID"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		if personalPhrases, ok := body["personalPhrases"]; ok {
			if pp, ok := personalPhrases.(string); !ok || pp == "" {
				err := getInvalidFieldError("personalPhrases", "string", body["personalPhrases"])
				SendServerResponse(w, nil, &ServerDebug{Error: err})
			} else {
				(*s.webinars)[eventID] = ServerWebinar{
					DeletionDate:    time.Now().Add(7 * 24 * time.Hour),
					PersonalPhrases: pp,
					Users:           (*s.webinars)[eventID].Users,
				}
				message := fmt.Sprintf("updated personal phrases for eventID %v: %+v", eventID, (*s.webinars)[eventID].PersonalPhrases)
				response := map[string]string{"message": message}
				SendServerResponse(w, response, nil)
			}
		} else {
			if email, ok := body["email"].(string); !ok || email == "" {
				err := getInvalidFieldError("email", "string", body["email"])
				SendServerResponse(w, nil, &ServerDebug{Error: err})
			} else if name, ok := body["name"].(string); !ok || name == "" {
				err := getInvalidFieldError("name", "string", body["name"])
				SendServerResponse(w, nil, &ServerDebug{Error: err})
			} else {
				response, debug := s.facecastLogin(eventID, email, name)
				SendServerResponse(w, response, debug)
			}
		}
	}
}

func (s *ServerApi) GetDashaMailData(w http.ResponseWriter, r *http.Request) {
	if body, err := ReadRequestBody(r.Body); err != nil {
		err := getBodyReadingError(err)
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if bookID, ok := body["bookID"].(string); !ok || bookID == "" {
		err := getInvalidFieldError("bookID", "string", body["bookID"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if emails, ok := body["emails"].([]interface{}); !ok || emails == nil {
		err := getInvalidFieldError("emails", "[]interface{}", body["emails"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		response, debug := s.getDashaMailData(bookID, emails)
		SendServerResponse(w, response, debug)
	}
}

func (s *ServerApi) CreateWebinarReport(w http.ResponseWriter, r *http.Request) {
	if body, err := ReadRequestBody(r.Body); err != nil {
		err := getBodyReadingError(err)
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if reportName, ok := body["reportName"].(string); !ok || reportName == "" {
		err := getInvalidFieldError("reportName", "string", body["reportName"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if reportData, ok := body["reportData"].([]interface{}); !ok || reportData == nil {
		err := getInvalidFieldError("reportData", "[]interface{}", body["reportData"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		debug := s.createWebinarReport(reportName, reportData, nil)
		SendServerResponse(w, nil, debug)
	}
}

func (s *ServerApi) CreateCampaignsReport(w http.ResponseWriter, r *http.Request) {
	if body, err := ReadRequestBody(r.Body); err != nil {
		err := getBodyReadingError(err)
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if reportName, ok := body["reportName"].(string); !ok || reportName == "" {
		err := getInvalidFieldError("reportName", "string", body["reportName"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if reportData, ok := body["reportData"].([]interface{}); !ok || reportData == nil {
		err := getInvalidFieldError("reportData", "[]interface{}", body["reportData"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		debug := s.createCampaignsReport(reportName, reportData, nil)
		SendServerResponse(w, nil, debug)
	}
}

func (s *ServerApi) SendDataToDashaMail(w http.ResponseWriter, r *http.Request) {
	if body, err := ReadRequestBody(r.Body); err != nil {
		err := getBodyReadingError(err)
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if bookID, ok := body["bookID"].(string); !ok || bookID == "" {
		err := getInvalidFieldError("bookID", "string", body["bookID"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else if infoDM, ok := body["infoDM"].(map[string]interface{}); !ok || infoDM == nil {
		err := getInvalidFieldError("infoDM", "map[string]interface{}", body["infoDM"])
		SendServerResponse(w, nil, &ServerDebug{Error: err})
	} else {
		debug := s.sendDataToDashaMail(bookID, infoDM, nil)
		SendServerResponse(w, nil, debug)
	}
}

func (s *ServerApi) HandleWebSocketConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}).Upgrade(w, r, nil)
	if err != nil {
		SendServerResponse(w, nil, &ServerDebug{Error: err}) // писать именно в w, а не в ws, т.к., видимо, при возникновении ошибки не происходит апгрейда w до ws
		return
	}

	var response interface{}
	debug := &ServerDebug{}
	wsWaiter := &WebSocketWaiter{
		Chan:     ws,
		Done:     make(chan struct{}),
		Ticker:   time.NewTicker(3 * time.Second), // частота комментариев, направляемых на frontend
		Response: &WebSocketWaiterResponse{Status: "processing"},
	}

	defer func() {
		wsWaiter.Ticker.Stop()
		wsWaiter.Done <- struct{}{}
		close(wsWaiter.Done)
		SendServerResponse(wsWaiter.Chan, response, debug)
		wsWaiter.Chan.Close() // закрыть канал по окончании работы функции
	}()

	var msg WebSocketMessageRequest
	err = ws.ReadJSON(&msg) // чтение нового сообщения в JSON-формате
	if err != nil {
		debug.Error = fmt.Errorf("error while reading WebSocket: %v", err)
		return
	}

	go waitingForServerValidAnswer(wsWaiter)
	validateData := func(msgData interface{}) (map[string]interface{}, bool) {
		data, ok := msgData.(map[string]interface{})
		return data, ok
	}

	switch msg.APIMethod {
	case "getWebinarReportInfo":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'eventID': 'string'}")
		} else if eventID, ok := data["eventID"].(string); !ok || eventID == "" {
			debug.Error = getInvalidFieldError("eventID", "string", data["eventID"])
		} else {
			response, debug = s.getWebinarReportInfo(eventID, wsWaiter.Response)
		}

	case "createWebinarReport":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'reportName': 'string', 'reportData': '[]interface{}'}")
		} else if reportName, ok := data["reportName"].(string); !ok || reportName == "" {
			debug.Error = getInvalidFieldError("reportName", "string", data["reportName"])
		} else if reportData, ok := data["reportData"].([]interface{}); !ok || reportData == nil {
			debug.Error = getInvalidFieldError("reportData", "[]interface{}", data["reportData"])
		} else {
			debug = s.createWebinarReport(reportName, reportData, wsWaiter.Response)
		}

	case "getCampaignsReportInfo":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'start_date': 'YYYY-MM-DD', 'end_date': 'YYYY-MM-DD'}")
		} else {
			var startDate, endDate string
			/*/
			 * Либо оба параметра - пустые строки, либо оба - непустые строки нужного формата.
			 * В первом случае endDate - текущая дата, а startDate - дата за 30 дней до текущей даты.
			/*/

			_, ok1 := data["start_date"]
			_, ok2 := data["end_date"]
			if !ok1 && !ok2 {
				now := time.Now().UTC()
				now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
				endDate = now.Format("2006-01-02")
				startDate = now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
			} else {
				startDate, ok1 = data["start_date"].(string)
				endDate, ok2 = data["end_date"].(string)
				if ok2 && endDate != "" && (!ok1 || startDate == "") {
					debug.Error = getInvalidFieldError("start_date", "format string 'YYYY-MM-DD'", data["start_date"])
				} else if ok1 && startDate != "" && (!ok2 || endDate == "") {
					debug.Error = getInvalidFieldError("end_date", "format string 'YYYY-MM-DD'", data["end_date"])
				}
			}

			if debug.Error == nil {
				response, debug = s.getCampaignsReportInfo(startDate, endDate, wsWaiter.Response)
			}
		}

	case "createCampaignsReport":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'reportName': 'string', 'reportData': '[]interface{}'}")
		} else if reportName, ok := data["reportName"].(string); !ok || reportName == "" {
			debug.Error = getInvalidFieldError("reportName", "string", data["reportName"])
		} else if reportData, ok := data["reportData"].([]interface{}); !ok || reportData == nil {
			debug.Error = getInvalidFieldError("reportData", "[]interface{}", data["reportData"])
		} else {
			debug = s.createCampaignsReport(reportName, reportData, wsWaiter.Response)
		}

	case "getCertificatesInfo":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'bookID': 'string'}")
		} else if bookID, ok := data["bookID"].(string); !ok || bookID == "" {
			debug.Error = getInvalidFieldError("bookID", "string", data["bookID"])
		} else {
			response, debug = s.getCertificatesInfo(bookID, wsWaiter.Response)
		}

	case "createCertificates":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'eventName': 'string', 'eventDate': 'string', 'usersInfo': 'map[string]interface{}'}")
		} else if eventName, ok := data["eventName"].(string); !ok || eventName == "" {
			debug.Error = getInvalidFieldError("eventName", "string", data["eventName"])
		} else if eventDate, ok := data["eventDate"].(string); !ok || eventDate == "" {
			debug.Error = getInvalidFieldError("eventDate", "string", data["eventDate"])
		} else if usersInfo, ok := data["usersInfo"].(map[string]interface{}); !ok || usersInfo == nil {
			debug.Error = getInvalidFieldError("usersInfo", "map[string]interface{}", data["usersInfo"])
		} else {
			response, debug = s.createCertificates(data, wsWaiter.Response)
		}

	case "sendDataToDashaMail":
		if data, ok := validateData(msg.Data); !ok {
			debug.Error = getDataValidFormatError("{'bookID': 'string', 'infoDM': 'map[string]interface{}'}")
		} else if bookID, ok := data["bookID"].(string); !ok || bookID == "" {
			debug.Error = getInvalidFieldError("bookID", "string", data["bookID"])
		} else if infoDM, ok := data["infoDM"].(map[string]interface{}); !ok || infoDM == nil {
			debug.Error = getInvalidFieldError("infoDM", "map[string]interface{}", data["infoDM"])
		} else {
			debug = s.sendDataToDashaMail(bookID, infoDM, wsWaiter.Response)
		}

	default:
		debug.Error = fmt.Errorf("unknown API method '%v'", msg.APIMethod)
	}
}

// EnableCORSRequests is an example middleware handler that enables CORS headers.
func (s *ServerApi) EnableCORSRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Add("Access-Control-Allow-Origin", "*")
		headers.Add("Vary", "Origin")
		headers.Add("Vary", "Access-Control-Request-Method")
		headers.Add("Vary", "Access-Control-Request-Headers")
		headers.Add("Access-Control-Allow-Headers", "Content-Type, Origin, Accept, token")
		headers.Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// EnableAuthentication is an example middleware handler that checks for a
// hardcoded bearer appToken. This can be used to verify session cookies, JWTs
// and more.
func (s *ServerApi) EnableAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuthentication := []string{} // add methods that require authentication here
		endpoint := strings.Split(strings.Split(r.URL.Path, "/api/v")[1], "/")[1]

		for _, authEndpoint := range requireAuthentication {
			if endpoint == authEndpoint {
				// make sure an Authorization header was provided
				appToken := r.Header.Get("Authorization")

				if appToken == "" {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				/*/
				 * This is where appToken validation would be done.
				 * For this boilerplate, we just check and make sure the appToken matches a hardcoded string.
				/*/
				appToken = strings.TrimPrefix(appToken, "Bearer ")
				if appToken != s.appToken {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}
			}
		}

		// assuming that passed, we can execute the authenticated handler
		next.ServeHTTP(w, r)
	})
}

type Handler struct {
	Port string
	http.Handler
}

// NewRouter returns an HTTP handler that implements the routes for the API.
func NewRouter() (Handler, error) {
	s := new(ServerApi)
	port, err := s.Init()

	r := chi.NewRouter()
	r.Use(s.EnableCORSRequests) //, s.EnableAuthentication)

	// register the API routes
	// GET requests
	r.Get("/{unknown}", s.UnknownEndpoint)
	r.Get("/getUserLK", s.GetUserLK)
	r.Get("/getUserPoints", s.GetUserPoints)
	r.Get("/getWebinarReportInfo", s.GetWebinarReportInfo)
	r.Get("/getCampaignsReportInfo", s.GetCampaignsReportInfo)

	// POST requests
	r.Post("/{unknown}", s.UnknownEndpoint)
	r.Post("/facecastLogin", s.FacecastLogin)
	r.Post("/getDashaMailData", s.GetDashaMailData)
	r.Post("/createWebinarReport", s.CreateWebinarReport)
	r.Post("/createCampaignsReport", s.CreateCampaignsReport)
	r.Post("/sendDataToDashaMail", s.SendDataToDashaMail)

	// WebSocket connections
	r.HandleFunc("/websocket", s.HandleWebSocketConnections)

	return Handler{port, r}, err
}
