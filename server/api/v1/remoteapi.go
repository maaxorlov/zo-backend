package v1

import (
	"encoding/json"
	"fmt"
	"github.com/lukasjarosch/go-docx"
	excel "github.com/xuri/excelize/v2"
	"html"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	. "zo-backend/server/api"
	yd "zo-backend/ya-disk"
)

func (s *ServerApi) getUserLK(email string) (*GetUserServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of getUserLK -> ")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getUserLK")

	titles, err := s.getBookTitles("82599", false, debug)
	if err != nil {
		return nil, debug
	}

	jsonData := s.getJSONBytes(DashaMailRequest{
		Method: "lists.get_members",
		Email:  email,
		BookID: "82599",
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, debug
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, debug
	}

	err = data.Data.CheckForOnlyOneElement(debug)
	if err != nil {
		return nil, debug
	}

	return setServerApiUserFields(data.Data[0], titles, debug), nil
}

// Параметр tildaView отвечает за то, какие параметры будут использованы в качестве ключей в возвращаемой карте.
// Вид параметров в DashaMail: merge_1, merge_2, ... , merge_i, где i - номер столбца (системные параметры).
// Вид параметров в Tilda: основная_специализация, event_name, ... ("человекопонятные" параметры).
// В качестве первого параметра функция возвращает map[string_1]string_2.
// Если tildaView == true, то string_1 в виде Tilda, а string_2 в виде DashaMail.
// Если tildaView == false, то string_1 в виде DashaMail, а string_2 в виде Tilda.
func (s *ServerApi) getBookTitles(bookID string, tildaView bool, debug *ServerDebug) (*map[string]string, error) {
	debug.SetDebugLastStage("getBookTitles -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	if bookID == "" {
		err = fmt.Errorf("not valid bookID")
		return nil, err
	}

	jsonData := s.getJSONBytes(DashaMailRequest{
		Method:     "lists.get",
		BookID:     bookID,
		JSONFormat: 1, // любой int вернет данные массивов в виде JSON-представления
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, err
	}

	err = data.Data.CheckForOnlyOneElement(debug)
	if err != nil {
		return nil, err
	}

	/*/
	 * Здесь и далее, скорее всего, это нечего не даст, так как если что-то произойдет с горутинами, то нигде не выведется мой Debug
	 * на досуге подумать, надо ли так делать
	/*/
	debug.SetDebugLastStage("group of goroutines")
	titles := make(map[string]string)
	respData := data.Data[0]
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(respData))

	for columnName := range respData {
		go func(columnName string) {
			defer wg.Done()

			if strings.Contains(columnName, "merge_") {
				val := DashaMailColumnTitle{}
				_ = json.Unmarshal([]byte(respData[columnName].(string)), &val)
				mu.Lock()
				if tildaView {
					titles[strings.ToLower(val.Title)] = strings.ToLower(columnName)
				} else {
					titles[strings.ToLower(columnName)] = strings.ToLower(val.Title)
				}
				mu.Unlock()
			}
		}(columnName)
	}
	wg.Wait()

	return &titles, nil
}

func (s *ServerApi) getUserPoints(email, pointsType string) (*GetUserPointsServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of getUserPoints -> ")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getUserPoints")

	if pointsType != "ZO" && pointsType != "NMO" {
		err = fmt.Errorf("unknown pointsType (only 'ZO' and 'NMO' are available)")
		return nil, debug
	}

	booksIDs, err := s.getAllBooks(debug)
	if err != nil {
		return nil, debug
	}

	nowQuarter, nowYear := getPointZOTimeFrames(pointsType)
	pointsInfo := GetUserPointsServerResponse{}
	data, err := s.getDashaMailDataForEmail(*booksIDs, email, debug, nil)
	if err != nil {
		return nil, debug
	}

	/*/
	 * по-хорошему, наверное, надо бы пройтись по циклу горутинами, но предполагается, что функция correctPoints()
	 * вернет true далеко не для всех элементов => как будто бы нет необходимости усложнять код асинхронностью
	/*/
	switch pointsType {
	case "NMO":
		for _, userPointsInfo := range *data {
			if correctPoints(userPointsInfo.NMO) {
				if dateInYear(userPointsInfo.EventDate, nowYear) {
					pointsNMO, _ := strconv.Atoi(userPointsInfo.ZET)
					pointsInfo.YearPoints += pointsNMO
				}

				if dateInQuarter(userPointsInfo.EventDate, nowYear, nowQuarter) {
					pointsNMO, _ := strconv.Atoi(userPointsInfo.ZET)
					pointsInfo.QuarterPoints += pointsNMO
				}

				pointsInfo.UserPointsInfo = append(pointsInfo.UserPointsInfo, UserPointsInfo{
					EventDate:   userPointsInfo.EventDate,
					EventName:   userPointsInfo.EventName,
					NMO:         userPointsInfo.NMO,
					ZET:         userPointsInfo.ZET,
					Certificate: userPointsInfo.Certificate,
				})
			}
		}

	case "ZO":
		for _, userPointsInfo := range *data {
			if pointsType == "ZO" && (correctPoints(userPointsInfo.PointsZOView) || correctPoints(userPointsInfo.PointsZOQuestion) || correctPoints(userPointsInfo.PointsZOPoll)) {
				if dateInYear(userPointsInfo.EventDate, nowYear) {
					pointsZOView, _ := strconv.Atoi(userPointsInfo.PointsZOView)
					pointsZOQuestion, _ := strconv.Atoi(userPointsInfo.PointsZOQuestion)
					pointsZOPool, _ := strconv.Atoi(userPointsInfo.PointsZOPoll)
					pointsInfo.YearPoints += pointsZOView + pointsZOQuestion + pointsZOPool
				}

				if dateInQuarter(userPointsInfo.EventDate, nowYear, nowQuarter) {
					pointsZOView, _ := strconv.Atoi(userPointsInfo.PointsZOView)
					pointsZOQuestion, _ := strconv.Atoi(userPointsInfo.PointsZOQuestion)
					pointsZOPool, _ := strconv.Atoi(userPointsInfo.PointsZOPoll)
					pointsInfo.QuarterPoints += pointsZOView + pointsZOQuestion + pointsZOPool

					pointsInfo.UserPointsInfo = append(pointsInfo.UserPointsInfo, UserPointsInfo{
						EventDate:        userPointsInfo.EventDate,
						EventName:        userPointsInfo.EventName,
						PointsZOView:     userPointsInfo.PointsZOView,
						PointsZOQuestion: userPointsInfo.PointsZOQuestion,
						PointsZOPoll:     userPointsInfo.PointsZOPoll,
					})
				}
			}
		}
	}

	return &pointsInfo, debug
}

func (s *ServerApi) getAllBooks(debug *ServerDebug) (*[]string, error) {
	debug.SetDebugLastStage("getAllBooks -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	jsonData := s.getJSONBytes(DashaMailRequest{
		Method:     "lists.get",
		JSONFormat: 1, // любой int вернет данные массивов в виде JSON-представления
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, err
	}

	booksIDs := make([]string, 0)
	for _, book := range data.Data {
		booksIDs = append(booksIDs, book["id"].(string))
	}

	return &booksIDs, nil
}

func (s *ServerApi) getDashaMailDataForEmail(booksIDs []string, email string, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) (*map[string]GetUserServerResponse, error) {
	debug.SetDebugLastStage("getDashaMailDataForEmail -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	errChan := initErrChan()
	goNum := initGoNum(len(booksIDs), 300)
	usersInfo := initSyncMap()
	debug.SetDebugLastStage("group of goroutines")

	for _, bookID := range booksIDs {
		go func(bookID string) {
			goNum.ControlMaxNum <- struct{}{}

			defer func() {
				setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("reading books' info from DashaMail: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
				<-goNum.ControlMaxNum
				calcGoNum(goNum, errChan)
			}()

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for bookID %v -> ", bookID))
			var user *GetUserServerResponse
			var err error
			var titles *map[string]string

			if errChan.OpenedState {
				titles, err = s.getBookTitles(bookID, false, localDebug)
				if err != nil {
					sendErrToErrChan(err, errChan, debug, localDebug)
				} else {
					user, err = s.readEmailData(bookID, titles, email, localDebug)
				}
			}

			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
			}

			if user != nil {
				addToSyncMap(usersInfo, bookID, *user)
			}
		}(bookID)
	}

	err = <-errChan.Chan
	if err != nil {
		return nil, err
	}

	u, err := DecodeToStruct((*map[string]GetUserServerResponse)(nil), usersInfo.Map, debug)
	if err != nil {
		return nil, fmt.Errorf("decoding interface{} to struct error: " + err.Error())
	}

	return u.(*map[string]GetUserServerResponse), nil
}

func (s *ServerApi) readEmailData(bookID string, titles *map[string]string, email string, debug *ServerDebug) (*GetUserServerResponse, error) {
	debug.SetDebugLastStage("readEmailData -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	jsonData := s.getJSONBytes(DashaMailRequest{
		Method: "lists.get_members",
		Email:  email,
		BookID: bookID,
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return &GetUserServerResponse{Message: "ошибка чтения: " + err.Error()}, nil
	}

	err = data.Data.CheckForOnlyOneElement(debug)
	if err != nil {
		return nil, err
	}

	return setServerApiUserFields(data.Data[0], titles, debug), nil
}

func (s *ServerApi) getWebinarReportInfo(eventID string, wsWaiterResp *WebSocketWaiterResponse) (*GetReportServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of getWebinarReportInfo -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started getting the report")

	var (
		report *GetReportServerResponse
		infoDM *map[string]GetUserServerResponse
	)

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getWebinarReportInfo")

	errChan := initErrChan()
	goNum := initGoNum(3)
	debug.SetDebugLastStage("group of goroutines")

	go func() {
		defer func() {
			setNewWSWaiterMessage(wsWaiterResp, "got webinar header")
			calcGoNum(goNum, errChan)
		}()

		localDebug := NewServerDebug(" -> ")
		var err error
		report, err = s.getWebinarHeader(eventID, localDebug)
		if err != nil {
			sendErrToErrChan(err, errChan, debug, localDebug)
		}
	}()

	usersChan := func() <-chan []UserInfo {
		ch := make(chan []UserInfo, 1)

		go func() {
			defer close(ch)

			localDebug := NewServerDebug(" -> ")
			users, err := s.getFacecastUsers(eventID, localDebug)
			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
				return
			}

			congressEmailsIndexes := make([]int, 0)
			for i := range users {
				if strings.Index(users[i].Email, "congresscentr.com") != -1 {
					congressEmailsIndexes = append(congressEmailsIndexes, i)
				}
			}

			/*/
			 * В обратном порядке, т.к. если в congressEmailsIndexes будет достаточно много элементов, а какой-то из последних близок
			 * к концу users, то при сокращении длины users будет ошибка доступа по индексу.
			 * Пример: в users элементы с "congresscentr.com" имеют индексы [0, 2, 3, 745], len(users) = 746.
			 * После удаления трех элементов из users с первыми тремя индексами из congressEmailsIndexes (т.е. 0, 2 и 3): len(users) = 743.
			 * При попытке получить users[745] будет ошибка.
			/*/
			for i := len(congressEmailsIndexes) - 1; i >= 0; i-- {
				users[congressEmailsIndexes[i]] = users[len(users)-1]
				users = users[:len(users)-1]
			}

			ch <- users
		}()

		return ch
	}()
	users := <-usersChan
	setNewWSWaiterMessage(wsWaiterResp, "got webinar viewers from FaceCast")

	go func() {
		defer func() {
			setNewWSWaiterMessage(wsWaiterResp, "got users' windows and minutes from FaceCast")
			calcGoNum(goNum, errChan)
		}()

		localDebug := NewServerDebug(" -> ")
		err := s.getUsersWindowsAndMinutes(&users, eventID, localDebug)
		if err != nil {
			sendErrToErrChan(err, errChan, debug, localDebug)
		}
	}()

	go func() {
		defer func() {
			setNewWSWaiterMessage(wsWaiterResp, "got users' info from DashaMail")
			calcGoNum(goNum, errChan)
		}()

		var emails []string
		for _, user := range users {
			emails = append(emails, user.Email)
		}

		localDebug := NewServerDebug(" -> ")
		var err error
		infoDM, err = s.getDashaMailDataForEmails("82599", emails, localDebug, wsWaiterResp)
		if err != nil {
			sendErrToErrChan(err, errChan, debug, localDebug)
		}
	}()

	err = <-errChan.Chan
	if err != nil {
		return nil, debug
	}

	eventMaxPoints := GetEventMaxPoints(report.EventInfo.VideoName) // получаем здесь отдельно один раз, чтобы не получать отдельно для каждого пользователя
	for _, user := range users {
		email := user.Email

		/*/
		 * Из-за технического сбоя одному и тому же пользователю могло быть выдано больше 1 ключа для просмотра трансляции => статистику
		 * необходимо собрать по всем просмотрам для одного пользователя. Если пользователя еще нет в отчете, то просто добавляем всю
		 * информацию по нему. Если же он уже был, то сохраняем все его уже записанные данные из ДМ, обновляя только кол-во всех окон
		 * и кол-во подтвержденных окон, информацию по минутам онлайн и офлайн, режим просмотра и кол=во бонусов ЗО (т.к. последние два
		 * поля зависят от информации по минутам и онлайн, и офлайн). К уже записанным минутам онлайн и офлайн дописываем новые минуты,
		 * которых не было до этого. Может быть так, что с разными ключами пользователь одну и ту же минуту посмотрел сначала в онлайне,
		 * а затем - в офлайне. Для этого сравниваем получившиеся массивы минут онлайн и офлайн, оставляя уникальные элементы в массиве
		 * минут офлайн (которых нет в массиве минут онлайн).
		/*/
		if _, ok := report.UsersInfo[strings.ToLower(email)]; ok {
			newUser := report.UsersInfo[strings.ToLower(email)]

			newUser.AllWindows += user.AllWindows // суммируем показанные пользователю окна
			newUser.Windows += user.Windows       // суммируем подтвержденные пользователем окна

			minutesOnlineChanged := AppendMinutesIfMissing(&newUser.MinutesOnline, user.MinutesOnline...) // дописываем новые онлайн минуты, которых не было до этого
			if minutesOnlineChanged {
				newUser.MinutesViewedOnline = len(newUser.MinutesOnline)                        // обновляем информацию по сумме минут онлайн
				newUser.FirstMinuteOnline = newUser.MinutesOnline[0]                            // обновляем информацию по первой минуте онлайн
				newUser.LastMinuteOnline = newUser.MinutesOnline[newUser.MinutesViewedOnline-1] // обновляем информацию по последней минуте онлайн
			}

			/*/
			 * Дописываем новые офлайн минуты, которых не было до этого, и оставляем уникальные
			 * элементы в массиве минут офлайн (которых нет в массиве минут онлайн).
			/*/
			minutesOfflineChanged := AppendMinutesIfMissing(&newUser.MinutesOffline, user.MinutesOffline...) && FilterMinutesOffline(&newUser.MinutesOffline, newUser.MinutesOnline...)
			if minutesOfflineChanged {
				newUser.MinutesViewedOffline = len(newUser.MinutesOffline)                         // обновляем информацию по сумме минут офлайн
				newUser.FirstMinuteOffline = newUser.MinutesOffline[0]                             // обновляем информацию по первой минуте офлайн
				newUser.LastMinuteOffline = newUser.MinutesOffline[newUser.MinutesViewedOffline-1] // обновляем информацию по последней минуте офлайн
			}

			newUser.ViewRegime = GetViewRegime(newUser.MinutesViewedOnline, newUser.MinutesViewedOffline)                                                    // перезаписываем режим просмотра, т.к. мог измениться
			newUser.PointsZOView = int(GetPointsZOView(eventMaxPoints, report.EventInfo.Duration, newUser.MinutesViewedOnline+newUser.MinutesViewedOffline)) // пересчитываем бонусы ЗО, т.к. могли измениться

			report.UsersInfo[strings.ToLower(email)] = newUser
		} else {
			user.Message = (*infoDM)[email].Message
			user.Name = (*infoDM)[email].Name
			user.Citizenship = (*infoDM)[email].Citizenship
			user.District = (*infoDM)[email].District
			user.Region = (*infoDM)[email].Region
			user.City = (*infoDM)[email].City
			user.Specialization = (*infoDM)[email].Specialization
			user.SpecializationExtra = (*infoDM)[email].SpecializationExtra
			user.Position = (*infoDM)[email].Position
			user.Own = (*infoDM)[email].Own
			user.ViewRegime = GetViewRegime(user.MinutesViewedOnline, user.MinutesViewedOffline)
			user.PointsZOView = int(GetPointsZOView(eventMaxPoints, report.EventInfo.Duration, user.MinutesViewedOnline+user.MinutesViewedOffline))
			if user.MinutesViewedOnline != 0 {
				user.FirstMinuteOnline = user.MinutesOnline[0]
				user.LastMinuteOnline = user.MinutesOnline[len(user.MinutesOnline)-1]
			}
			if user.MinutesViewedOffline != 0 {
				user.FirstMinuteOffline = user.MinutesOffline[0]
				user.LastMinuteOffline = user.MinutesOffline[len(user.MinutesOffline)-1]
			}
			if user.ViewRegime != "" {
				report.EventInfo.ViewersTotal++
			}

			report.UsersInfo[strings.ToLower(email)] = user
		}
	}

	//fmt.Printf("---REPORT: %+v\n", report.UsersInfo)

	return report, nil
}

func (s *ServerApi) getCampaignsReportInfo(startDate, endDate string, wsWaiterResp *WebSocketWaiterResponse) (*[]interface{}, *ServerDebug) {
	debug := NewServerDebug("start of getCampaignsReportInfo -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started getting the report")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getCampaignsReportInfo")

	campaignsMainInfo, err := s.getCampaignsMainInfo(startDate, endDate, debug)
	if err != nil {
		return nil, debug
	}

	errChan := initErrChan()
	goNum := initGoNum(len(*campaignsMainInfo), 300)
	campaignsReport := initSyncArray()
	debug.SetDebugLastStage("group of goroutines")

	for i, campaignMainInfo := range *campaignsMainInfo {
		go func(i int, campaignMainInfo DashaMailCampaignReport) {
			goNum.ControlMaxNum <- struct{}{}

			defer func() {
				setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("reading detailed campaigns' info from DashaMail: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
				<-goNum.ControlMaxNum
				calcGoNum(goNum, errChan)
			}()

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for campaign %v with id %v -> ", campaignMainInfo.Name, campaignMainInfo.ID))
			var campaignDetailedInfo *DashaMailCampaignReport
			var err error
			if errChan.OpenedState {
				campaignDetailedInfo, err = s.getCampaignDetailedInfo(campaignMainInfo.ID, localDebug)
			}

			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
			}

			if campaignDetailedInfo != nil {
				atoi := func(a string) int { i, _ := strconv.Atoi(a); return i }
				addToSyncArray(campaignsReport, CampaignReport{
					Date:              campaignMainInfo.Date,
					Name:              html.UnescapeString(campaignMainInfo.Name),
					TagUTM:            campaignMainInfo.TagUTM,
					SourceUTM:         campaignMainInfo.SourceUTM,
					MediumUTM:         campaignMainInfo.MediumUTM,
					ContentUTM:        campaignMainInfo.ContentUTM,
					TermUTM:           campaignMainInfo.TermUTM,
					Sent:              atoi(campaignDetailedInfo.Sent),
					Clicked:           atoi(campaignDetailedInfo.Clicked),
					Opened:            atoi(campaignDetailedInfo.Opened),
					FirstSent:         campaignDetailedInfo.FirstSent,
					FirstOpen:         campaignDetailedInfo.FirstOpen,
					LastOpen:          campaignDetailedInfo.LastOpen,
					FirstClick:        campaignDetailedInfo.FirstClick,
					LastClick:         campaignDetailedInfo.LastClick,
					UniqueOpened:      atoi(campaignDetailedInfo.UniqueOpened),
					UniqueClicked:     atoi(campaignDetailedInfo.UniqueClicked),
					Unsubscribed:      atoi(campaignDetailedInfo.Unsubscribed),
					SpamComplained:    atoi(campaignDetailedInfo.SpamComplained),
					SpamBlocked:       atoi(campaignDetailedInfo.SpamBlocked),
					SpamMarked:        atoi(campaignDetailedInfo.SpamMarked),
					MailSystemBlocked: atoi(campaignDetailedInfo.MailSystemBlocked),
					Hard:              atoi(campaignDetailedInfo.Hard),
					Soft:              atoi(campaignDetailedInfo.Soft),
				})
			}
		}(i, campaignMainInfo)
	}

	err = <-errChan.Chan
	if err != nil {
		return nil, debug
	}

	sort.Slice(campaignsReport.Array, func(i, j int) bool {
		earlier, _ := time.Parse("2006-01-02 15:04:05", campaignsReport.Array[j].(CampaignReport).Date)
		later, _ := time.Parse("2006-01-02 15:04:05", campaignsReport.Array[i].(CampaignReport).Date)
		return later.After(earlier)
	})

	//fmt.Printf("---DATA: %+v\n", campaignsReport.Array)

	return &campaignsReport.Array, nil
}

func (s *ServerApi) getCampaignsMainInfo(startDate, endDate string, debug *ServerDebug) (*[]DashaMailCampaignReport, error) {
	debug.SetDebugLastStage("getCampaignsMainInfo -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	_, err = time.Parse("2006-01-02", startDate)
	if err != nil {
		err = fmt.Errorf("error startDate format (need 'YYYY-MM-DD')")
		return nil, err
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		err = fmt.Errorf("error endDate format (need 'YYYY-MM-DD')")
		return nil, err
	}
	/*/
	 * Из поддержки ДМ про фильтр delivery_time при status="SENT":
	 * "start - больше 0 часов 0 минут 0 секунд указанного дня, end - меньше 0 часов 0 минут 0 секунд указанного дня."
	 * Значит, как будто, для включения end в фильтр необходимо добавить к end один день (24 часа).
	/*/
	endDate = end.Add(24 * time.Hour).Format("2006-01-02")

	jsonData := s.getJSONBytes(DashaMailRequest{
		Method:     "campaigns.get",
		Status:     "SENT",
		StartDate:  startDate,
		EndDate:    endDate,
		JSONFormat: 1, // любой int вернет данные массивов в виде JSON-представления
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, err
	}

	campaignsMainInfo := make([]DashaMailCampaignReport, 0, len(data.Data))
	for _, mainInfo := range data.Data {
		var m interface{}
		m, err = DecodeToStruct((*DashaMailCampaignReport)(nil), mainInfo, debug)
		if err != nil {
			err = fmt.Errorf("decoding interface{} to struct error: " + err.Error())
			return nil, err
		}

		campaignsMainInfo = append(campaignsMainInfo, *m.(*DashaMailCampaignReport))
	}

	return &campaignsMainInfo, nil
}

func (s *ServerApi) getCampaignDetailedInfo(campaignID string, debug *ServerDebug) (*DashaMailCampaignReport, error) {
	debug.SetDebugLastStage("getCampaignDetailedInfo -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	_campaignID, err := strconv.Atoi(campaignID)
	if err != nil {
		return nil, err
	}
	jsonData := s.getJSONBytes(DashaMailRequest{
		Method:     "reports.summary",
		CampaignID: _campaignID,
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, err
	}

	campaignReport := struct {
		Response struct {
			Data *DashaMailCampaignReport `json:"data"`
		} `json:"response"`
	}{}
	err = json.Unmarshal(response, &campaignReport)
	if err != nil {
		return nil, err
	}

	return campaignReport.Response.Data, nil
}

func (s *ServerApi) getCertificatesInfo(bookID string, wsWaiterResp *WebSocketWaiterResponse) (*GetCertificatesInfoServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of getCertificatesInfo -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started getting the certificates info for all users")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getCertificatesInfo")

	infoDM, err := s.getDashaMailDataForBook(bookID, debug, wsWaiterResp)
	if err != nil {
		return nil, debug
	}

	certificatesInfo := &GetCertificatesInfoServerResponse{UsersInfo: make(map[string]CertificatePersonalInfo)}
	setNewWSWaiterMessage(wsWaiterResp, "started getting the certificates info for users with NMO codes")
	debug.SetDebugLastStage("getting the certificates info")

	for user, info := range *infoDM {
		userType := getUserType(info)
		if userType != "" {
			errParam := checkUserValidity(info, userType)
			if errParam != "" {
				err = fmt.Errorf("required user %s has empty '%s' (try to add '%s' for this user in DM book '%s' and restart)", user, errParam, errParam, bookID)
				return nil, debug
			}

			setGeneralCertificatesInfo(certificatesInfo, info)
			certificatesInfo.UsersInfo[user] = *setPersonalCertificatesInfo(info, userType)
		}
	}

	if len(certificatesInfo.UsersInfo) == 0 {
		err = fmt.Errorf("there are no users with NMO code or offline students without certificates in DM book '%s'", bookID)
		return nil, debug
	}

	err = checkGeneralCertificatesInfo(certificatesInfo, debug)
	if err != nil {
		return nil, debug
	}

	return certificatesInfo, debug
}

func (s *ServerApi) createCertificates(data map[string]interface{}, wsWaiterResp *WebSocketWaiterResponse) (map[string]interface{}, *ServerDebug) {
	debug := NewServerDebug("start of createCertificates -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started creating certificates")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of createCertificates")

	_certificatesInfo, err := DecodeToStruct((*GetCertificatesInfoServerResponse)(nil), data, debug)
	if err != nil {
		err = fmt.Errorf("decoding interface{} to struct error: " + err.Error())
		return nil, debug
	}
	certificatesInfo := _certificatesInfo.(*GetCertificatesInfoServerResponse)

	err = s.createDOCXCertificates(certificatesInfo, debug, wsWaiterResp)
	if err != nil {
		return nil, debug
	}

	certificatesLocalDir := certificatesInfo.EventDate
	err = s.convertDOCX2PDF(certificatesLocalDir, debug, wsWaiterResp)
	if err != nil {
		return nil, debug
	}

	setNewWSWaiterMessage(wsWaiterResp, "started loading certificates to Yandex Disk")
	d, err := yd.InitYaDisk(s.yaDiskAcc.ApiKey)
	if err != nil {
		return nil, debug
	}

	err = (&YaDisk{d}).checkYaDiskFoldersValidity("Сертификаты НМО/{year}/{month}/{eventDate}", certificatesInfo.EventDate, debug)
	if err != nil {
		return nil, debug
	}

	infoDM, err := (&YaDisk{d}).loadCertificatesToYaDisk(certificatesLocalDir, debug, wsWaiterResp)
	if err != nil {
		return nil, debug
	}

	return infoDM, nil
}

func (s *ServerApi) createDOCXCertificates(certificatesInfo *GetCertificatesInfoServerResponse, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) error {
	debug.SetDebugLastStage("createDOCXCertificates -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	errChan := initErrChan()
	goNum := initGoNum(len(certificatesInfo.UsersInfo))
	debug.SetDebugLastStage("group of goroutines")

	for user, userInfo := range certificatesInfo.UsersInfo {
		go func(userEmail string, userInfo CertificatePersonalInfo) {
			defer func() {
				setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("creating certificates: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
				calcGoNum(goNum, errChan)
			}()

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for email %v -> ", userEmail))
			var err error
			if errChan.OpenedState {
				err = s.createDOCXCertificate(userEmail, certificatesInfo.EventName, certificatesInfo.EventDate, userInfo, localDebug)
			}

			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
			}
		}(user, userInfo)
	}

	err = <-errChan.Chan

	return err
}

func (s *ServerApi) createDOCXCertificate(userEmail, eventName, eventDate string, userInfo CertificatePersonalInfo, debug *ServerDebug) error {
	debug.SetDebugLastStage("createDOCXCertificate")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	replaceMap := docx.PlaceholderMap{
		"ИМЯ":         userInfo.UserName,
		"МЕРОПРИЯТИЕ": eventName,
		"ДАТА":        eventDate,
	}

	if userInfo.ZET == "" {
		replaceMap["АКАДЕМ"] = userInfo.AcademicHours
		replaceMap["ШАБЛОН"] = "./__dev__certificates__/template_students.docx"
	} else {
		replaceMap["НМО"] = userInfo.NMO
		replaceMap["ЗЕТ"] = userInfo.ZET
		replaceMap["ШАБЛОН"] = "./__dev__certificates__/template_adults.docx"
	}

	doc, err := docx.Open(replaceMap["ШАБЛОН"].(string))
	if err != nil {
		return err
	}
	defer doc.Close()

	err = doc.ReplaceAll(replaceMap)
	if err != nil {
		return err
	}

	err = doc.WriteToFile(fmt.Sprintf("./%s/Сертификат НМО для %s.docx", eventDate, userEmail))
	if err != nil {
		return err
	}

	return nil
}

func (s *ServerApi) convertDOCX2PDF(certificatesLocalDir string, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) error {
	debug.SetDebugLastStage("convertDOCX2PDF -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	setNewWSWaiterMessage(wsWaiterResp, "converting certificates")
	debug.SetDebugLastStage("getting go-script directory")
	wd, err := getDir()
	if err != nil {
		return err
	}

	debug.SetDebugLastStage("writing to '.path' file")
	err = writePathFile(wd, certificatesLocalDir)
	if err != nil {
		return err
	}

	debug.SetDebugLastStage("running 'docx2pdf.exe' converter")
	err = exec.Command(wd + "/__dev__certificates__/docx2pdf.exe").Run()
	if err != nil {
		err = fmt.Errorf("running 'docx2pdf.exe' converter error: %v", err)
		return err
	}

	return nil
}

type YaDisk struct{ *yd.YaDisk }

func (d *YaDisk) checkYaDiskFoldersValidity(destinationFolder, eventDate string, debug *ServerDebug) error {
	debug.SetDebugLastStage("checkYaDiskFoldersValidity -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	debug.SetDebugLastStage("checkEventDateValidity")
	month, year, err := checkEventDateValidity(eventDate)
	if err != nil {
		return err
	}

	destinationFolder = strings.Replace(destinationFolder, "{year}", year, -1)
	destinationFolder = strings.Replace(destinationFolder, "{month}", month, -1)
	destinationFolder = strings.Replace(destinationFolder, "{eventDate}", eventDate, -1)
	underlyingFolders := strings.Split(destinationFolder, "/")
	folder := ""

	debug.SetDebugLastStage("checkYaDiskFolderValidity")
	for i := range underlyingFolders {
		folder += underlyingFolders[i] + "/"
		err = d.checkYaDiskFolderValidity(folder)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *YaDisk) checkYaDiskFolderValidity(folderName string) error {
	checkYaDiskError := func(err error) error {
		switch {
		case strings.HasSuffix(err.Error(), "DiskNotFoundError"):
			return nil
		case strings.HasSuffix(err.Error(), "DiskPathPointsToExistentDirectoryError"):
			return nil
		default:
			return err
		}
	}

	err := d.GetYaDiskFolder(folderName, false)
	if err != nil {
		err = checkYaDiskError(err)
		if err != nil {
			return err
		}

		err = d.CreateYaDiskFolder(folderName)
		if err != nil {
			err = checkYaDiskError(err)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *YaDisk) loadCertificatesToYaDisk(certificatesLocalDir string, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) (map[string]interface{}, error) {
	debug.SetDebugLastStage("loadCertificatesToYaDisk -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	filesFromDir, err := ioutil.ReadDir(filepath.Join(".", certificatesLocalDir))
	if err != nil {
		return nil, err
	}

	errChan := initErrChan()
	goNum := initGoNum(len(filesFromDir) / 2) // в папке точно половина .docx, половина - .pdf, иначе ранее вернется ошибка при конвертации
	loadedFiles := initSyncMap()
	unloadedFiles := initSyncMap()
	month, year, _ := checkEventDateValidity(certificatesLocalDir) // можно не проверять ошибку, т.к. выше уже проверялась валидность этой даты
	certificatesRemoteDir := filepath.Join("Сертификаты НМО", year, month, certificatesLocalDir)
	debug.SetDebugLastStage("group of goroutines")

	for _, file := range filesFromDir {
		go func(fileName string) {
			if filepath.Ext(fileName) == ".pdf" {
				defer func() {
					setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("uploading certificates to YD: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
					calcGoNum(goNum, errChan)
				}()

				localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for file %v -> ", fileName))
				var loadedFileInfo YaDiskLoadedFileInfo
				if errChan.OpenedState {
					loadedFileInfo = d.loadFileToYaDisk(fileName, certificatesLocalDir, certificatesRemoteDir, localDebug)
				}

				pdfFilePath := filepath.Join(certificatesLocalDir, fileName)
				docxFilePath := strings.Replace(pdfFilePath, ".pdf", ".docx", -1)
				email := strings.TrimSuffix(strings.TrimPrefix(fileName, "Сертификат НМО для "), ".pdf")

				err := os.Remove(docxFilePath) // удаляем файл с расширением .docx
				if err != nil {
					sendErrToErrChan(err, errChan, debug, localDebug)
					return
				}

				if loadedFileInfo.Error != nil {
					// если файл СОЗДАН локально И НЕ ЗАГРУЖЕН на ЯД, то записать в созданные и незагруженные файлы (такие файлы позже можно будет переносить на ЯД вручную при необходимости)
					unloadedFileInfo := struct {
						FileName string `json:"fileName"`
						Error    string `json:"error"`
					}{
						FileName: fileName,
						Error:    loadedFileInfo.Error.Error(),
					}
					addToSyncMap(unloadedFiles, email, unloadedFileInfo)
				} else {
					// если файл СОЗДАН локально И ЗАГРУЖЕН на ЯД, то записать в созданные и загруженные файлы
					err := os.Remove(pdfFilePath) // удаляем файл с расширением .pdf
					if err != nil {
						sendErrToErrChan(err, errChan, debug, localDebug)
						return
					}

					loadedFileInfo := struct {
						Link string `json:"link"`
					}{
						Link: loadedFileInfo.Link,
					}
					addToSyncMap(loadedFiles, email, loadedFileInfo)
				}
			}
		}(file.Name())
	}

	err = <-errChan.Chan
	if err != nil {
		return nil, err
	}

	// если ВСЕ файлы СОЗДАНЫ локально И ЗАГРУЖЕНЫ на ЯД, то удаляем локальную папку для хранения сертификатов на конкретную дату
	if len(unloadedFiles.Map) == 0 {
		err = os.Remove(certificatesLocalDir)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{"links": loadedFiles.Map, "unloadedFiles": unloadedFiles.Map}, nil
}

func (d *YaDisk) loadFileToYaDisk(fileName, localDir, remoteDir string, debug *ServerDebug) YaDiskLoadedFileInfo {
	debug.SetDebugLastStage("loadFileToYaDisk")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	link, err := d.LoadToYaDisk(fileName, localDir, remoteDir, true)
	if err != nil {
		return YaDiskLoadedFileInfo{Error: err}
	}

	return YaDiskLoadedFileInfo{Link: link}
}

func (s *ServerApi) getDashaMailDataForBook(bookID string, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) (*map[string]GetUserServerResponse, error) {
	debug.SetDebugLastStage("getDashaMailDataForBook -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	debug.SetDebugLastStage("formation of request parameters")
	infoDM := make(map[string]GetUserServerResponse)
	if err != nil {
		err = fmt.Errorf("bookID '%v' must be of type int", bookID)
		return nil, err
	}
	jsonData := s.getJSONBytes(DashaMailRequest{
		Method: "lists.get_members",
		BookID: bookID,
		Limit:  1e6, // любое большое число (лишь бы больше максимального количества в книге), иначе вернется информация не по всем пользователям в книге
	})

	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return nil, err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return nil, err
	}

	titles, err := s.getBookTitles(bookID, false, debug)
	if err != nil {
		return nil, err
	}

	for num, user := range data.Data {
		setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("readting users info in book %v: %v done, %v left", bookID, num, len(data.Data)-num))
		infoDM[user["email"].(string)] = *setServerApiUserFields(user, titles, debug)
	}

	return &infoDM, nil
}

func (s *ServerApi) getWebinarHeader(eventID string, debug *ServerDebug) (*GetReportServerResponse, error) {
	debug.SetDebugLastStage("getWebinarHeader -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	params := url.Values{}
	params.Add("uid", s.facecastAcc.ApiKey)
	params.Add("api_key", s.facecastAcc.ApiSecret)
	params.Add("event_code", eventID)

	uri := fmt.Sprintf("%sv1/get_event?%s", s.facecastAcc.URI, params.Encode())
	response, err := DoRequestGET(uri, debug)
	if err != nil {
		return nil, err
	}

	var report GetReportServerResponse
	err = json.Unmarshal(response, &report.EventInfo)
	if err != nil {
		return nil, fmt.Errorf(err.Error()+" Server response: "+string(response), http.StatusBadRequest)
	}

	report.EventInfo.StartDate = ISOToHuman(report.EventInfo.StartDate)
	report.EventInfo.PlanStartDate = ISOToHuman(report.EventInfo.PlanStartDate)
	report.EventInfo.EndDate = ISOToHuman(report.EventInfo.EndDate)
	report.EventInfo.TimeViewersMax = ISOToHuman(report.EventInfo.TimeViewersMax)
	report.EventInfo.ReportTime = TimeToHuman(time.Now())
	report.EventInfo.Duration = int(math.Ceil(float64(report.EventInfo.Duration) / 60))          // в минуты и округление в бОльшую сторону
	report.ReportName = "Отчёт " + strings.Replace(report.EventInfo.PlanStartDate, ":", ".", -1) // заменяем в названии формат времени с 00:00 на 00.00 для корректного сохранения файлов в дальнейшем
	report.UsersInfo = make(map[string]UserInfo)

	return &report, nil
}

// проверить, какие поля {из Name, Email, Key, WayToAdd, EventID} мне действительно нужны при возврате
func (s *ServerApi) getFacecastUsers(eventID string, debug *ServerDebug) ([]UserInfo, error) {
	debug.SetDebugLastStage("getFacecastUsers -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	params := url.Values{}
	params.Add("uid", s.facecastAcc.ApiKey)
	params.Add("api_key", s.facecastAcc.ApiSecret)
	params.Add("event_code", eventID)

	uri := s.facecastAcc.URI + "v1/get_event_tickets"
	response, err := DoRequestPOST(uri, []byte(params.Encode()), debug)
	if err != nil {
		return nil, err
	}

	var _err FacecastErrorResponse
	_ = json.Unmarshal(response, &_err)
	if _err.Error != "" {
		err = fmt.Errorf("%s", _err.Error)
		return nil, err
	}

	var tickets []FacecastTicketResponse
	err = json.Unmarshal(response, &tickets)
	if err != nil {
		return nil, fmt.Errorf("%s Server response: %s", err.Error(), string(response))
	}

	uri = s.facecastAcc.URI + "v1/get_event_keys"
	response, err = DoRequestPOST(uri, []byte(params.Encode()), debug)
	if err != nil {
		return nil, err
	}

	var keys []FacecastKeyResponse
	err = json.Unmarshal(response, &keys)
	if err != nil {
		return nil, fmt.Errorf("%s Server response: %s", err.Error(), string(response))
	}

	users := make([]UserInfo, 0)
	singleKeys := 0

	serviceKeys := make([]FacecastKeyResponse, 0)
	serviceTickets := make(map[int64]FacecastTicketResponse, 0) // без email, созданные через менеджер зрителей в ЛК ФК
	for _, k := range keys {
		isSingle := true
		if k.TicketID == 0 {
			singleKeys++
			continue
		}
		for _, t := range tickets {
			if k.TicketID == t.TicketID && t.Email != "" {
				isSingle = false
				users = append(users, UserInfo{
					Name:     t.Name,
					Email:    t.Email,
					Key:      k.Key,
					WayToAdd: "server",
					EventID:  eventID,
				})
				break
			}
			if _, ok := serviceTickets[t.TicketID]; t.Email == "" && !ok {
				serviceTickets[t.TicketID] = t
			}
		}
		if isSingle {
			serviceKeys = append(serviceKeys, k)
		}
	}

	//// проверка отсутствие юзеров без почт
	//if len(serviceTickets) != 0 {
	//	var tickets []FacecastTicketResponse
	//	for _, ticket := range serviceTickets {
	//		tickets = append(tickets, ticket)
	//	}
	//	err = fmt.Errorf("have %v users without email: %+v", len(tickets), tickets)
	//	return nil, err
	//}

	// проверка на совпадение количества ключей и юзеров
	if len(tickets)-len(serviceTickets) != len(keys)-singleKeys {
		err = fmt.Errorf("have some mismatches with users and keys: len(tickets) %v len(keys) %v serviceKeys %+v", len(tickets), len(keys), serviceKeys)
		return nil, err
	}

	return users, nil
}

func (s *ServerApi) getUsersWindowsAndMinutes(users *[]UserInfo, eventID string, debug *ServerDebug) error {
	debug.SetDebugLastStage("getUsersWindowsAndMinutes -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	params := url.Values{}
	params.Add("uid", s.facecastAcc.ApiKey)
	params.Add("api_key", s.facecastAcc.ApiSecret)
	params.Add("event_code", eventID)

	uri := s.facecastAcc.URI + "v1/get_visit_stats" // минуты
	response, err := DoRequestPOST(uri, []byte(params.Encode()), debug)
	if err != nil {
		return err
	}

	var _err FacecastErrorResponse
	_ = json.Unmarshal(response, &_err)
	if _err.Error != "" {
		err = fmt.Errorf("%s", _err.Error)
		return err
	}

	var minutes []FacecastMinutesResponse
	err = json.Unmarshal(response, &minutes)
	if err != nil {
		return fmt.Errorf("%s Server response: %s", err.Error(), string(response))
	}

	uri = s.facecastAcc.URI + "v1/get_user_activity_detailed_all" // окна
	response, err = DoRequestPOST(uri, []byte(params.Encode()), debug)
	if err != nil {
		return err
	}

	var windows []FacecastWindowsResponse
	err = json.Unmarshal(response, &windows)
	if err != nil {
		return fmt.Errorf("%s Server response: %s", err.Error(), string(response))
	}

	//// проверка на совпадение количества юзеров и окон
	//if len(*users) != len(windows) {
	//	err = fmt.Errorf("Have some mismatches with users and windows!!! len(users) %v len(windows) %v", len(*users), len(windows))
	//	return err
	//}

	//// проверка на совпадение количества юзеров и гистограмм
	//if len(*users) != len(minutes) {
	//	err = fmt.Errorf("Have some mismatches with users and minutes!!! len(users) %v len(minutes) %v", len(*users), len(minutes))
	//	return err
	//}

	for userNum, userInfo := range *users {
		for _, m := range minutes {
			if m.Key == userInfo.Key {
				for _, minuteInfo := range m.Minutes {
					if len(minuteInfo.Views) != 0 {
						if online := minuteInfo.Views[0].(map[string]interface{})["is_live"].(bool); online {
							(*users)[userNum].MinutesViewedOnline += 1
							(*users)[userNum].MinutesOnline = append((*users)[userNum].MinutesOnline, minuteInfo.Position+1)
						} else {
							(*users)[userNum].MinutesViewedOffline += 1
							(*users)[userNum].MinutesOffline = append((*users)[userNum].MinutesOffline, minuteInfo.Position+1)
						}
					}
				}
				break
			}
		}

		for _, w := range windows {
			if w.Key == userInfo.Key {
				(*users)[userNum].AllWindows += 1
				if w.Confirmed == 1 {
					(*users)[userNum].Windows += 1
				}
			}
		}
	}

	return nil
}

func (s *ServerApi) getDashaMailDataForEmails(bookID string, emails []string, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) (*map[string]GetUserServerResponse, error) {
	debug.SetDebugLastStage("getDashaMailDataForEmails -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	titles, err := s.getBookTitles(bookID, false, debug)
	if err != nil {
		return nil, err
	}

	errChan := initErrChan()
	goNum := initGoNum(len(emails), 300)
	usersInfo := initSyncMap()
	debug.SetDebugLastStage("group of goroutines")

	for _, email := range emails {
		go func(email string) {
			goNum.ControlMaxNum <- struct{}{}

			defer func() {
				setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("reading users' info from DashaMail: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
				<-goNum.ControlMaxNum
				calcGoNum(goNum, errChan)
			}()

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for email %v -> ", email))
			var user *GetUserServerResponse
			var err error
			if errChan.OpenedState {
				user, err = s.readEmailData(bookID, titles, email, localDebug)
			}

			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
			}

			if user != nil {
				addToSyncMap(usersInfo, email, *user)
			}
		}(email)
	}

	err = <-errChan.Chan
	if err != nil {
		return nil, err
	}

	_usersInfo, err := DecodeToStruct((*map[string]GetUserServerResponse)(nil), usersInfo.Map, debug)
	if err != nil {
		return nil, fmt.Errorf("decoding interface{} to struct error: " + err.Error())
	}

	return _usersInfo.(*map[string]GetUserServerResponse), nil
}

func (s *ServerApi) facecastLogin(eventID, email, name string) (*FacecastLoginServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of facecastLogin -> ")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of facecastLogin")

	/*/
	 * Далее на протяжении блока работаем с локальной копией информации по конкретному вебинару => для сохранения возможных изменений
	 * необходимо (возможно) измененную копию вписать в map перед выходом из функции
	/*/
	webinar := new(ServerWebinar)
	*webinar = (*s.webinars)[eventID]
	defer func() { (*s.webinars)[eventID] = *webinar }()

	if webinar.Users.PreviousQueryTime == nil || webinar.Users.PreviousQueryTime.Before(time.Now().Add(-1*time.Hour)) {
		var users []UserInfo
		users, err = s.getFacecastUsers(eventID, debug)
		if err != nil {
			return nil, debug
		}

		for _, user := range users {
			webinar.Users.Info = append(webinar.Users.Info, ServerUser{
				Email:    user.Email,
				Name:     user.Name,
				Key:      user.Key,
				WayToAdd: user.WayToAdd,
			})
		}

		now := time.Now()
		webinar.Users.PreviousQueryTime = &now
		webinar.DeletionDate = time.Now().Add(7 * 24 * time.Hour)
	}

	for _, user := range webinar.Users.Info {
		if user.Email == email {
			return &FacecastLoginServerResponse{Key: user.Key, PersonalPhrases: webinar.PersonalPhrases}, nil
		}
	}

	key := s.generateKey(email)
	params := url.Values{}
	params.Add("uid", s.facecastAcc.ApiKey)
	params.Add("api_key", s.facecastAcc.ApiSecret)
	params.Add("event_code", eventID)
	params.Add("name", name)
	params.Add("email", email)
	params.Add("key", key)
	params.Add("multiple_vpp", "0")

	uri := s.facecastAcc.URI + "v1/insert_key"
	response, err := DoRequestPOST(uri, []byte(params.Encode()), debug)
	if err != nil {
		return nil, debug
	}

	var insertKey FacecastInsertKeyResponse
	err = json.Unmarshal(response, &insertKey)
	if err != nil {
		err = fmt.Errorf("%s; Facecast server response: %s", err.Error(), response)
		return nil, debug
	}

	if !insertKey.Success {
		err = fmt.Errorf("can't insert key because not success Facecast server response: %s", response)
		return nil, debug
	}

	webinar.Users.Info = append(webinar.Users.Info, ServerUser{
		Email:    email,
		Name:     name,
		Key:      key,
		WayToAdd: "manual",
	})
	webinar.DeletionDate = time.Now().Add(7 * 24 * time.Hour)

	return &FacecastLoginServerResponse{Key: key, PersonalPhrases: webinar.PersonalPhrases}, nil
}

func (s *ServerApi) getDashaMailData(bookID string, emails []interface{}) (*map[string]GetUserServerResponse, *ServerDebug) {
	debug := NewServerDebug("start of getDashaMailData -> ")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of getDashaMailData")

	/*/
	 * Нельзя просто вернуть саму функцию, т.к. если внутри нее будет ошибка, а до нее ошибок не было, то err не перезапишется
	 * и останется nil, тогда и executionStages не запишет путь до ошибки.
	/*/

	var strEmails []string
	for i, email := range emails {
		if strEmail, ok := email.(string); !ok {
			err = fmt.Errorf("email %v has wrong type %T (want string)", email, email)
			return nil, debug
		} else if strEmail == "" {
			err = fmt.Errorf("got empty email in emails (check index %v in emails starting indexing with 0)", i)
			return nil, debug
		} else {
			strEmails = append(strEmails, strEmail)
		}
	}
	response, err := s.getDashaMailDataForEmails(bookID, strEmails, debug, nil)

	return response, debug
}

func (s *ServerApi) createWebinarReport(reportName string, reportData []interface{}, wsWaiterResp *WebSocketWaiterResponse) *ServerDebug {
	debug := NewServerDebug("start of createWebinarReport -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started creating the report")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of createWebinarReport")

	setNewWSWaiterMessage(wsWaiterResp, "started creating the excel report")
	err = s.writeReportData(reportName, "webinar", reportData, debug, wsWaiterResp)
	if err != nil {
		return debug
	}

	setNewWSWaiterMessage(wsWaiterResp, "started loading the excel report to Yandex Disk")
	d, err := yd.InitYaDisk(s.yaDiskAcc.ApiKey)
	if err != nil {
		return debug
	}

	eventDate := strings.Join(strings.Split(strings.Split(reportName, " ")[1], "."), " ")
	err = (&YaDisk{d}).checkYaDiskFoldersValidity("Отчёты по мероприятиям/{year}/{month}/", eventDate, debug)
	if err != nil {
		return debug
	}

	month, year, _ := checkEventDateValidity(eventDate) // можно не проверять ошибку, т.к. выше уже проверялась валидность этой даты
	remoteDir := filepath.Join("Отчёты по мероприятиям", year, month)
	loadedFileInfo := (&YaDisk{d}).loadFileToYaDisk(reportName+".xlsx", "", remoteDir, debug)
	if loadedFileInfo.Error != nil {
		err = fmt.Errorf("can't load file %s to Yandex Disk: %+v", reportName+".xlsx", loadedFileInfo.Error)
		return debug
	}
	err = os.Remove(reportName + ".xlsx")

	return debug
}

func (s *ServerApi) writeReportData(reportName, reportType string, reportData []interface{}, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) error {
	debug.SetDebugLastStage("writeReportData -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	f := excel.NewFile()
	f.SetSheetName("Sheet1", "Отчёт")

	switch reportType {
	case "webinar":
		// построение статистики просмотров и распределений по минутам и специальностям по данным в excel-отчете
		setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("writing excel report info"))
		viewingRegimesChartData, specialisationsChartData, minutesDistributionChartData, _err := WriteExcelWebinarData(f, reportData, debug) // запись данных в excel-отчет и сбор данных для графиков
		if _err != nil {
			err = fmt.Errorf("writing error for file %s: %+v", reportName, _err)
			return err
		}

		// построение графиков excel-отчета
		setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("plotting excel charts"))
		err = PlotWebinarCharts(f, viewingRegimesChartData, specialisationsChartData, minutesDistributionChartData, debug)
		if err != nil {
			err = fmt.Errorf("can't plot charts for file %s: %+v", reportName, err)
			return err
		}

	case "campaigns":
		// построение excel-отчета по рассылкам
		setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("writing excel report info"))
		err = WriteExcelCampaignsData(f, reportData, debug)
		if err != nil {
			err = fmt.Errorf("writing error for file %s: %+v", reportName, err)
			return err
		}
	}

	// настройка стилей excel-отчета
	err = AutoResizeColumns(f, "Отчёт", debug)
	if err != nil {
		return err
	}

	// сохранение excel-отчета
	debug.SetDebugLastStage("saving an excel report")
	if err = f.SaveAs(reportName + ".xlsx"); err != nil {
		err = fmt.Errorf("saving error for file %s: %+v", reportName, err)
		return err
	}

	return nil
}

func (s *ServerApi) createCampaignsReport(reportName string, reportData []interface{}, wsWaiterResp *WebSocketWaiterResponse) *ServerDebug {
	debug := NewServerDebug("start of createCampaignsReport -> ")
	setNewWSWaiterMessage(wsWaiterResp, "started creating the report")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of createCampaignsReport")

	setNewWSWaiterMessage(wsWaiterResp, "started creating the excel report")
	err = s.writeReportData(reportName, "campaigns", reportData, debug, wsWaiterResp)
	if err != nil {
		return debug
	}

	setNewWSWaiterMessage(wsWaiterResp, "started loading the excel report to Yandex Disk")
	d, err := yd.InitYaDisk(s.yaDiskAcc.ApiKey)
	if err != nil {
		return debug
	}

	err = (&YaDisk{d}).checkYaDiskFoldersValidity("Отчёты по рассылкам/{year}", time.Now().Format("02 01 2006"), debug)
	if err != nil {
		return debug
	}

	remoteDir := filepath.Join("Отчёты по рассылкам", fmt.Sprintf("%v", time.Now().Year()))
	loadedFileInfo := (&YaDisk{d}).loadFileToYaDisk(reportName+".xlsx", "", remoteDir, debug)
	if loadedFileInfo.Error != nil {
		err = fmt.Errorf("can't load file %s to Yandex Disk: %+v", reportName+".xlsx", loadedFileInfo.Error)
		return debug
	}
	err = os.Remove(reportName + ".xlsx")

	return debug
}

func (s *ServerApi) updateDashaMailData(bookID string, infoDM map[string]interface{}, debug *ServerDebug, wsWaiterResp *WebSocketWaiterResponse) (*map[string]string, error) {
	debug.SetDebugLastStage("updateDashaMailData -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	i, err := DecodeToStruct((*map[string]DashaMailUpdateInfo)(nil), infoDM, debug)
	if err != nil {
		return nil, fmt.Errorf("decoding interface{} to struct error: " + err.Error())
	}
	_infoDM := *(i.(*map[string]DashaMailUpdateInfo))

	titles, err := s.getBookTitles(bookID, true, debug)
	if err != nil {
		return nil, err
	}

	errChan := initErrChan()
	goNum := initGoNum(len(infoDM), 300)
	writingDMLogs := initSyncMap()
	debug.SetDebugLastStage("group of goroutines")

	for email, userInfo := range _infoDM {
		go func(email string, userInfo DashaMailUpdateInfo) {
			goNum.ControlMaxNum <- struct{}{}

			defer func() {
				setNewWSWaiterMessage(wsWaiterResp, fmt.Sprintf("updating DashaMail users' info: %v done, %v left", goNum.Counter, goNum.Num-goNum.Counter))
				<-goNum.ControlMaxNum
				calcGoNum(goNum, errChan)
			}()

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for email %v -> ", email))
			var err error
			if errChan.OpenedState {
				columnsNames, params := getColumnsNamesAndParams(userInfo, titles)
				err = s.addUserToDashaMailBook(bookID, titles, email, columnsNames, params, localDebug)
			}

			if err != nil {
				if strings.Index(err.Error(), "DashaMail error") != -1 {
					addToSyncMap(writingDMLogs, email, "ошибка записи: "+err.Error())
				} else {
					sendErrToErrChan(err, errChan, debug, localDebug)
				}
			}
		}(email, userInfo)
	}

	err = <-errChan.Chan
	if err != nil {
		return nil, err
	}

	invalidEmails, err := DecodeToStruct((*map[string]string)(nil), writingDMLogs.Map, debug)
	if err != nil {
		return nil, fmt.Errorf("decoding interface{} to struct error: " + err.Error())
	}

	return invalidEmails.(*map[string]string), nil
}

func (s *ServerApi) addUserToDashaMailBook(bookID string, titles *map[string]string, email string, columnsNames []string, params []interface{}, debug *ServerDebug) error {
	debug.SetDebugLastStage("addUserToDashaMailBook -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	d := DashaMailRequest{
		Method:  "lists.add_member",
		BookID:  bookID,
		Email:   email,
		Update:  "update",   // любая string перезапишет данные для существующего email
		NoCheck: "no_check", // любая string впишет email в книгу в DashaMail без валидации
	}

	errChan := initErrChan()
	goNum := initGoNum(len(columnsNames))
	debug.SetDebugLastStage("group of goroutines")
	for i := 0; i < len(columnsNames); i++ {
		go func(i int) {
			defer calcGoNum(goNum, errChan)

			localDebug := NewServerDebug(fmt.Sprintf(" -> goroutine for columnsNames %v -> ", columnsNames[i]))
			var err error
			if errChan.OpenedState {
				err = setDashaMailFieldParam(&d, columnsNames[i], params[i], titles, localDebug)
			}

			if err != nil {
				sendErrToErrChan(err, errChan, debug, localDebug)
			}
		}(i)
	}

	err = <-errChan.Chan
	if err != nil {
		return err
	}

	jsonData := s.getJSONBytes(d)
	response, err := DoRequestPOST(s.dashaMailAcc.URI, jsonData, debug)
	if err != nil {
		return err
	}

	data := UnmarshalResponseData(response)

	err = data.Msg.CheckForError(debug)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServerApi) sendDataToDashaMail(bookID string, infoDM map[string]interface{}, wsWaiterResp *WebSocketWaiterResponse) *ServerDebug {
	debug := NewServerDebug("start of sendDataToDashaMail -> ")

	var err error
	defer debug.SetDebugFinalStage(&err, "end of sendDataToDashaMail")

	invalidEmails, err := s.updateDashaMailData(bookID, infoDM, debug, wsWaiterResp)
	if err == nil {
		err = checkInvalidEmailsErr(invalidEmails, debug)
	}

	return debug
}
