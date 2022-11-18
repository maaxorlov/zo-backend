package v1

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dchest/siphash"
	. "zo-backend/server/api"
)

func getBodyReadingError(err error) error {
	return fmt.Errorf("error during reading body: %s (need map[string]interface{})", err.Error())
}

func getInvalidFieldError(fieldName, neededType string, receivedData ...interface{}) error {
	if len(receivedData) == 0 {
		return fmt.Errorf("'%s': empty field or invalid field type (need %s)", fieldName, neededType)
	}
	return fmt.Errorf("'%s': empty field or invalid field type (need %s got %T)", fieldName, neededType, receivedData[0])
}

func getDataValidFormatError(dataValidFormat string) error {
	return fmt.Errorf("check data valid format (should be %s)", dataValidFormat)
}

// Добавляет API-ключ и формирует JSON из данных для запроса.
func (s *ServerApi) getJSONBytes(d DashaMailRequest) []byte {
	d.APIKey = s.dashaMailAcc.ApiKey
	jsonData, _ := json.Marshal(d)

	return jsonData
}

func waitingForServerValidAnswer(wsWaiter *WebSocketWaiter) {
	for {
		select {
		case <-wsWaiter.Done:
			return
		case <-wsWaiter.Ticker.C:
			SendServerResponse(wsWaiter.Chan, wsWaiter.Response, nil)
		}
	}
}

func setNewWSWaiterMessage(wsWaiterResp *WebSocketWaiterResponse, message string) {
	if wsWaiterResp != nil {
		wsWaiterResp.Message = message
	}
}

func setServerApiUserFields(userDM map[string]interface{}, titles *map[string]string, debug *ServerDebug) *GetUserServerResponse {
	debug.SetDebugLastStage("setServerApiUserFields")
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)
	user := new(GetUserServerResponse)

	for field, param := range userDM {
		if param == "" || (!strings.Contains(field, "merge_") && field != "state") {
			continue
		}

		wg.Add(1)
		go func(field string, param interface{}) {
			defer wg.Done()
			setServerApiUserFieldParam(user, field, param, titles, mu)
		}(field, param)
	}
	wg.Wait()

	return user
}

func setServerApiUserFieldParam(user *GetUserServerResponse, field string, param interface{}, titles *map[string]string, mu *sync.Mutex) {
	stringVal := param.(string)

	if field == "state" {
		mu.Lock()
		user.Status = setStatusExplain(stringVal)
		user.StatusExplain = stringVal
		mu.Unlock()
	} else {
		structField := new(string)
		switch (*titles)[field] {
		case "name":
			structField = &user.Name
		case "phone":
			structField = &user.Phone
		case "гражданство":
			structField = &user.Citizenship
		case "федеральный_округ":
			structField = &user.District
		case "регион_для_россии":
			structField = &user.Region
		case "город":
			structField = &user.City
		case "основная_медицинская_специализация":
			structField = &user.Specialization
		case "дополнительная_медицинская_специализация":
			structField = &user.SpecializationExtra
		case "место_работы":
			structField = &user.WorkPlace
		case "ваша_должность":
			structField = &user.Position
		case "свои":
			structField = &user.Own

		// сертификаты
		case "event_date":
			structField = &user.EventDate
		case "event_name":
			structField = &user.EventName
		case "код_нмо":
			structField = &user.NMO
		case "зет":
			structField = &user.ZET
		case "ссылка_на_сертификат":
			structField = &user.Certificate
		case "тип_посещения":
			structField = &user.VisitationType
		case "академические_часы":
			structField = &user.AcademicHours

		// баллы ЗО
		case "бонусы_зо_за_просмотр":
			structField = &user.PointsZOView
		case "бонусы_зо_за_вопрос":
			structField = &user.PointsZOQuestion
		case "бонусы_зо_за_опрос":
			structField = &user.PointsZOPoll
		}

		mu.Lock()
		*structField = html.UnescapeString(stringVal)
		mu.Unlock()
	}
}

func setStatusExplain(status string) int {
	// 'active','unsubscribed','bounced','inactive','unconfirmed' - активный, отписавшийся, неверный, неактивный, неподтвержденный
	switch status {
	case "active":
		return 1
	case "unsubscribed":
		return 2
	case "bounced":
		return 3
	case "inactive":
		return 4
	case "unconfirmed":
		return 5
	default:
		return -1
	}
}

func getPointZOTimeFrames(pointsType string) (int, string) {
	if pointsType == "ZO" {
		now := time.Now()
		return getQuarter(now.Month().String()), strconv.Itoa(now.Year())
	} else {
		return 0, ""
	}
}

func getQuarter(month string) int {
	switch month {
	case "января", "January", "февраля", "February", "марта", "March":
		return 1
	case "апреля", "April", "мая", "May", "июня", "June":
		return 2
	case "июля", "July", "августа", "August", "сентября", "September":
		return 3
	case "октября", "October", "ноября", "November", "декабря", "December":
		return 4
	default:
		return -1
	}
}

func correctPoints(points string) bool {
	if points != "" && points != "0" {
		return true
	} else {
		return false
	}
}

func dateInQuarter(date, nowYear string, nowQuarter int) bool {
	splt := strings.Split(date, " ")
	if len(splt) > 1 && splt[1] != "" {
		month := splt[1]
		if getQuarter(month) == nowQuarter && dateInYear(date, nowYear) {
			return true
		}
	}

	return false
}

func dateInYear(date string, nowYear string) bool {
	splt := strings.Split(date, " ")
	if len(splt) > 2 && splt[2] != "" {
		year := splt[2]
		if year == nowYear {
			return true
		}
	}

	return false
}

func updatePersonalPhrasesDates(webinars *map[string]ServerWebinar) {
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)

	wg.Add(len(*webinars))
	for eventID, webinar := range *webinars {
		go func(eventID string, webinar ServerWebinar) {
			defer wg.Done()

			if time.Now().After(webinar.DeletionDate) {
				mu.Lock()
				delete(*webinars, eventID)
				mu.Unlock()
			}
		}(eventID, webinar)
	}
	wg.Wait()
}

// Инициализация указателя на структуру ErrChan{}.
// Поле OpenedState отвечает за открытое состояние канала ошибок errChan.Chan. Переходит в false при ошибке в какой-либо горутине. На каждом
// этапе выполнения (в каждой горутине) поле проверяется, и если оно false, то горутина не выполняется и прекращает свое исполнение. Это ускоряет
// переход к разблокировке основного потока программы (в месте чтения из errChan.Chan). Если в канал ошибок errChan.Chan попадает ошибка err при
// выполнении какой-то горутины, то она сразу считывается и возвращается из внешней функции без ожидания окончания работы оставшихся горутин.
// Канал ошибок errChan.Chan закроется одним из отложенных вызовов calcGoNum().
func initErrChan() *ErrChan {
	return &ErrChan{
		Chan:        make(chan error, 1),
		OpenedState: true,
		Locker:      new(sync.Mutex),
	}
}

// Инициализация указателя на структуру GoNum{}.
// Поле Counter отвечает за счетчик завершивших свою работу горутин.
// Поле Num отвечает за общее количество запускаемых горутин.
// Поле ControlMaxNum отвечает за канал, контролирующий максимальное количество одновременно запускаемых горутин (если в программе
// запускается Num > MaxNum горутин, то одновременно могут выполняться не более MaxNum горутин, остальные ждут в очереди. После завершения
// хотя бы одной горутины начинает выполняться следующая из очереди).
func initGoNum(num int, maxNum ...int) *GoNum {
	goNum := &GoNum{
		Num: num,
	}

	if len(maxNum) != 0 {
		goNum.ControlMaxNum = make(chan struct{}, maxNum[0])
	}

	return goNum
}

// Закрытие канала ошибок errChan.Chan при достижении счетчика завершивших свою работу горутин goNum.Counter максимального значения goNum.MaxNum.
func calcGoNum(goNum *GoNum, errChan *ErrChan) {
	goNum.Locker.Lock()
	defer goNum.Locker.Unlock()
	goNum.Counter++
	if goNum.Counter == goNum.Num {
		close(errChan.Chan)
	}
}

func initSyncArray() *SyncArray {
	return &SyncArray{
		Array: make([]interface{}, 0),
	}
}

func addToSyncArray(syncArray *SyncArray, value interface{}) {
	syncArray.Locker.Lock()
	syncArray.Array = append(syncArray.Array, value)
	syncArray.Locker.Unlock()
}

func initSyncMap() *SyncMap {
	return &SyncMap{
		Map: make(map[string]interface{}),
	}
}

func addToSyncMap(syncMap *SyncMap, key string, value interface{}) {
	syncMap.Locker.Lock()
	syncMap.Map[key] = value
	syncMap.Locker.Unlock()
}

func sendErrToErrChan(err error, errChan *ErrChan, debug, localDebug *ServerDebug) {
	errChan.Locker.Lock()
	defer errChan.Locker.Unlock()

	if errChan.OpenedState {
		debug.ExecutionStages += localDebug.ExecutionStages
		debug.LastResponseData = localDebug.LastResponseData
		debug.LastSentData = localDebug.LastSentData

		errChan.Chan <- err
		errChan.OpenedState = false
	}
}

func (s *ServerApi) generateKey(text string) string {
	h := siphash.New([]byte(s.facecastAcc.ApiSecret[:16]))
	h.Write([]byte(text))

	return hex.EncodeToString(h.Sum(nil))
}

func checkInvalidEmailsErr(invalidEmails *map[string]string, debug *ServerDebug) error {
	debug.SetDebugLastStage("checkInvalidEmailsErr")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	if len(*invalidEmails) != 0 {
		i := 0
		info := ""
		for email, emailErr := range *invalidEmails {
			i++
			info += fmt.Sprintf("%v) EMAIL: %s\nERROR: %v\n", i, email, emailErr)
		}

		return fmt.Errorf("invalid emails:\n%s", info)
	}

	return nil
}

func getColumnsNamesAndParams(userInfo DashaMailUpdateInfo, titles *map[string]string) ([]string, []interface{}) {
	allColumnsNames := [...]string{
		// report data fields
		"окон_показано", "окон_подтверждено", "просмотрено_минут_в_эфире", "просмотрено_минут_в_записи",
		"кодировка_мероприятия", "бонусы_зо_за_просмотр", "бонусы_зо_за_вопрос", "бонусы_зо_за_опрос", "режим_просмотра",

		// user lk and user event registration data fields
		"name", "phone", "гражданство", "федеральный_округ", "регион_для_россии", "город",
		"основная_медицинская_специализация", "дополнительная_медицинская_специализация", "место_работы", "ваша_должность",

		// user event registration data fields
		"event_name", "event_date", "event_format", "тип_посещения",

		// utm tags
		"utm_source", "utm_medium", "utm_campaign", "utm_content",

		// certificate data fields
		"ссылка_на_сертификат",
	}
	columnsNames := make([]string, 0)
	params := make([]interface{}, 0)

	for _, fieldName := range allColumnsNames {
		if _, ok := (*titles)[fieldName]; ok {
			if param := getFieldInfo(userInfo, fieldName); param != nil {
				columnsNames = append(columnsNames, fieldName)
				params = append(params, param)
			}
		}
	}

	return columnsNames, params
}

func getFieldInfo(userInfo DashaMailUpdateInfo, fieldName string) interface{} {
	switch fieldName {
	// report data fields
	case "окон_показано":
		return userInfo.WindowsShowed
	case "окон_подтверждено":
		return userInfo.WindowsConfirmed
	case "просмотрено_минут_в_эфире":
		return userInfo.MinutesOnline
	case "просмотрено_минут_в_записи":
		return userInfo.MinutesOffline
	case "кодировка_мероприятия":
		return userInfo.EventCodeName
	case "бонусы_зо_за_просмотр":
		return userInfo.PointsZOView
	case "бонусы_зо_за_вопрос":
		return userInfo.PointsZOQuestion
	case "бонусы_зо_за_опрос":
		return userInfo.PointsZOPool
	case "режим_просмотра":
		return userInfo.ViewRegime

	// user lk and user event registration data fields
	case "name":
		return userInfo.Name
	case "phone":
		return userInfo.Phone
	case "гражданство":
		return userInfo.Citizenship
	case "федеральный_округ":
		return userInfo.District
	case "регион_для_россии":
		return userInfo.Region
	case "город":
		return userInfo.City
	case "основная_медицинская_специализация":
		return userInfo.Specialization
	case "дополнительная_медицинская_специализация":
		return userInfo.SpecializationExtra
	case "место_работы":
		return userInfo.WorkPlace
	case "ваша_должность":
		return userInfo.Position

	// user event registration data fields
	case "event_name":
		return userInfo.EventName
	case "event_date":
		return userInfo.EventDate
	case "event_format":
		return userInfo.EventFormat
	case "тип_посещения":
		return userInfo.VisitationType

	// utm tags
	case "utm_source":
		return userInfo.SourceUTM
	case "utm_medium":
		return userInfo.MediumUTM
	case "utm_campaign":
		return userInfo.CampaignUTM
	case "utm_content":
		return userInfo.ContentUTM

	// certificate data fields
	case "ссылка_на_сертификат":
		return userInfo.Link
	}

	return nil
}

func setGeneralCertificatesInfo(certificatesInfo *GetCertificatesInfoServerResponse, info GetUserServerResponse) {
	if certificatesInfo.EventName == "" && info.EventName != "" {
		certificatesInfo.EventName = info.EventName
	}
	if certificatesInfo.EventDate == "" && info.EventDate != "" {
		certificatesInfo.EventDate = info.EventDate
	}
}

func getUserType(info GetUserServerResponse) string {
	userType := ""
	if info.NMO != "" && info.Certificate == "" { // добавляем участников с НМО, у которых еще ранее не создан сертификат, или ранее создан с ошибкой => не был добавлен в ДМ
		userType = ADULT
	}
	if info.VisitationType == "Очное посещение" && info.Position == "Студент" && info.Certificate == "" { // добавляем студентов, посещавших мероприятие очно, у которых еще ранее не создан сертификат, или ранее создан с ошибкой => не был добавлен в ДМ
		userType = STUDENT
	}

	return userType
}

func checkUserValidity(info GetUserServerResponse, userType string) string {
	switch {
	case info.Name == "":
		return "name"
	case info.EventName == "":
		return "event_name"
	case info.EventDate == "":
		return "event_date"
	default:
		switch {
		case userType == ADULT && info.ZET == "":
			return "зет"
		case userType == STUDENT && info.AcademicHours == "":
			return "академические_часы"
		default:
			return ""
		}
	}
}

func setPersonalCertificatesInfo(info GetUserServerResponse, userType string) *CertificatePersonalInfo {
	switch userType {
	case ADULT:
		return &CertificatePersonalInfo{
			UserName: info.Name,
			ZET:      info.ZET,
			NMO:      info.NMO,
		}
	case STUDENT:
		return &CertificatePersonalInfo{
			UserName:      info.Name,
			AcademicHours: info.AcademicHours,
		}
	default:
		return nil
	}
}

func checkGeneralCertificatesInfo(certificatesInfo *GetCertificatesInfoServerResponse, debug *ServerDebug) error {
	debug.SetDebugLastStage("checkGeneralCertificatesInfo")

	emptyParam := ""
	switch {
	case certificatesInfo.EventName == "":
		emptyParam = "event name"
	case certificatesInfo.EventDate == "":
		emptyParam = "event date"
	default:
		return nil
	}

	return fmt.Errorf("empty %s", emptyParam)
}

func checkEventDateValidity(eventDate string) (string, string, error) {
	splt := strings.Split(eventDate, " ")
	if len(splt) < 3 {
		return "", "", fmt.Errorf("check event date format (must be like '01 января 1900' OR '01 01 1900' but got %s)", eventDate)
	}

	_, err := strconv.Atoi(splt[0])
	if err != nil {
		return "", "", fmt.Errorf("check event date day: %+v", err)
	}

	month := checkEventDateMonth(splt[1])
	if month == "" {
		return "", "", fmt.Errorf("check event date month: %+v", err)
	}

	year := splt[2]
	_, err = strconv.Atoi(year)
	if err != nil {
		return "", "", fmt.Errorf("check event date year: %+v", err)
	}

	return month, year, nil
}

func checkEventDateMonth(month string) string {
	switch month {
	case "января", "01":
		return "январь"
	case "февраля", "02":
		return "февраль"
	case "марта", "03":
		return "март"
	case "апреля", "04":
		return "апрель"
	case "мая", "05":
		return "май"
	case "июня", "06":
		return "июнь"
	case "июля", "07":
		return "июль"
	case "августа", "08":
		return "август"
	case "сентября", "09":
		return "сентябрь"
	case "октября", "10":
		return "октябрь"
	case "ноября", "11":
		return "ноябрь"
	case "декабря", "12":
		return "декабрь"
	default:
		return ""
	}
}

func getDir() (string, error) {
	wd, err := os.Getwd()
	wd = strings.Replace(strings.Replace(wd, ":\\", "://", 1), "\\", "/", -1)
	if err != nil {
		return "", err
	}

	return wd, nil
}

func writePathFile(wd, eventDate string) error {
	f, err := os.Create(wd + "/__dev__certificates__/.path")
	if err != nil {
		return err
	}

	path := filepath.Join(wd, eventDate)
	_, err = f.WriteString(fmt.Sprintf("CERTIFICATES_PATH=\"%s\"", path))
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return nil
}

func setDashaMailFieldParam(d *DashaMailRequest, columnName string, columnValue interface{}, titles *map[string]string, debug *ServerDebug) error {
	debug.SetDebugLastStage("setDashaMailFieldParam")
	if columnValue == "" {
		return nil
	}

	switch (*titles)[columnName] {
	case "merge_1":
		d.Field1 = columnValue
	case "merge_2":
		d.Field2 = columnValue
	case "merge_3":
		d.Field3 = columnValue
	case "merge_4":
		d.Field4 = columnValue
	case "merge_5":
		d.Field5 = columnValue
	case "merge_6":
		d.Field6 = columnValue
	case "merge_7":
		d.Field7 = columnValue
	case "merge_8":
		d.Field8 = columnValue
	case "merge_9":
		d.Field9 = columnValue
	case "merge_10":
		d.Field10 = columnValue
	case "merge_11":
		d.Field11 = columnValue
	case "merge_12":
		d.Field12 = columnValue
	case "merge_13":
		d.Field13 = columnValue
	case "merge_14":
		d.Field14 = columnValue
	case "merge_15":
		d.Field15 = columnValue
	case "merge_16":
		d.Field16 = columnValue
	case "merge_17":
		d.Field17 = columnValue
	case "merge_18":
		d.Field18 = columnValue
	case "merge_19":
		d.Field19 = columnValue
	case "merge_20":
		d.Field20 = columnValue
	case "merge_21":
		d.Field21 = columnValue
	case "merge_22":
		d.Field22 = columnValue
	case "merge_23":
		d.Field23 = columnValue
	case "merge_24":
		d.Field24 = columnValue
	case "merge_25":
		d.Field25 = columnValue
	case "merge_26":
		d.Field26 = columnValue
	case "merge_27":
		d.Field27 = columnValue
	case "merge_28":
		d.Field28 = columnValue
	case "merge_29":
		d.Field29 = columnValue
	case "merge_30":
		d.Field30 = columnValue
	case "merge_31":
		d.Field31 = columnValue
	case "merge_32":
		d.Field32 = columnValue
	case "merge_33":
		d.Field33 = columnValue
	case "merge_34":
		d.Field34 = columnValue
	case "merge_35":
		d.Field35 = columnValue
	default:
		return fmt.Errorf("%v: unknown or unnecessary value struct field name %v", columnName, (*titles)[columnName])
	}

	return nil
}
