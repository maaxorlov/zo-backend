package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func SendServerResponse(w interface{}, response interface{}, debug *ServerDebug) {
	if response == nil {
		response = ""
	}

	if fmt.Sprintf("%T", w) == "*middleware.compressResponseWriter" {
		if debug != nil && debug.Error != nil {
			response := ErrorMessageServerResponse{Message: getErrorMessage(debug)}
			JsonResponse(w.(http.ResponseWriter), response, http.StatusBadRequest)
		} else {
			JsonResponse(w.(http.ResponseWriter), response, http.StatusOK)
		}
	}

	if fmt.Sprintf("%T", w) == "*websocket.Conn" {
		if debug != nil && debug.Error != nil {
			response := ErrorMessageServerResponse{Message: getErrorMessage(debug)}
			err := w.(*websocket.Conn).WriteJSON(response) // send new message to the WebSocket channel
			LogWebSocketError(err)
		} else {
			err := w.(*websocket.Conn).WriteJSON(response) // send new message to the WebSocket channel
			LogWebSocketError(err)
		}
	}
}

// If a message is sent while websocket connection is closing, ignore the error
func UnsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}

func JsonResponse(w http.ResponseWriter, data interface{}, c int) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Error creating JSON response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)

	fmt.Fprintf(w, "%s", jsonData)
}

func ReadRequestBody(r io.Reader) (map[string]interface{}, error) {
	decoder := json.NewDecoder(r)
	// используется для хранения параметра ключ = значение данных
	var body map[string]interface{}
	// анализ параметров и сохранение в карте
	err := decoder.Decode(&body)

	return body, err
}

func httpRequest(request *http.Request, debug *ServerDebug) ([]byte, error) {
	debug.SetDebugLastStage("httpRequest")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func DoRequestGET(uri string, debug *ServerDebug) ([]byte, error) {
	debug.SetDebugLastStage("DoRequestGET -> ")

	var err error
	var response []byte
	jsonBytes := []byte(uri)
	defer func() {
		debug.DeleteDebugLastStage(&err)
		debug.SetDebugData(&jsonBytes, &response)
	}()

	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err = httpRequest(request, debug)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func DoRequestPOST(uri string, jsonBytes []byte, debug *ServerDebug) ([]byte, error) {
	debug.SetDebugLastStage("DoRequestPOST -> ")

	var err error
	var response []byte
	defer func() {
		debug.DeleteDebugLastStage(&err)
		debug.SetDebugData(&jsonBytes, &response)
	}()

	body := bytes.NewReader(jsonBytes)
	request, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	// выглядит, как костыль; на досуге подумать об изменениях
	if strings.Contains(uri, "facecast") {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	response, err = httpRequest(request, debug)
	if err != nil {
		return nil, err
	}

	return response, nil
}
