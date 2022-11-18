package api

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ADULT   = "ADULT"
	STUDENT = "STUDENT"
)

type ErrorMessageServerResponse struct {
	Message string `json:"message"`
}

type ServerAccInfo struct {
	URI       string
	ApiKey    string
	ApiSecret string
}

type ServerDebug struct {
	Error            error
	ExecutionStages  string
	LastResponseData []byte
	LastSentData     []byte
}

type ServerWebinar struct {
	DeletionDate    time.Time // храним 7 дней
	PersonalPhrases string
	Users           ServerUsers
}

type ServerUsers struct {
	Info              []ServerUser
	PreviousQueryTime *time.Time
}

type ServerUser struct {
	Email    string
	Name     string
	Key      string
	WayToAdd string
}

type ErrChan struct {
	Chan        chan error
	OpenedState bool
	Locker      *sync.Mutex
}

type GoNum struct {
	Counter       int
	Num           int
	ControlMaxNum chan struct{}
	Locker        sync.Mutex
}

type SyncArray struct {
	Array  []interface{}
	Locker sync.RWMutex
}

type SyncMap struct {
	Map    map[string]interface{}
	Locker sync.RWMutex
}

type DashaMailColumnTitle struct {
	Title string `json:"title"`
}

type DashaMailRequest struct {
	Method     string `json:"method"`
	APIKey     string `json:"api_key"`
	Email      string `json:"email,omitempty"`
	BookID     string `json:"list_id,omitempty"`
	NoCheck    string `json:"no_check,omitempty"`
	Update     string `json:"update,omitempty"`
	CampaignID int    `json:"campaign_id,omitempty"`
	Status     string `json:"status"`
	StartDate  string `json:"start,omitempty"`
	EndDate    string `json:"end,omitempty"`
	Limit      int64  `json:"limit"`
	JSONFormat int64  `json:"merge_json,omitempty"`

	Field1  interface{} `json:"merge_1,omitempty"`
	Field2  interface{} `json:"merge_2,omitempty"`
	Field3  interface{} `json:"merge_3,omitempty"`
	Field4  interface{} `json:"merge_4,omitempty"`
	Field5  interface{} `json:"merge_5,omitempty"`
	Field6  interface{} `json:"merge_6,omitempty"`
	Field7  interface{} `json:"merge_7,omitempty"`
	Field8  interface{} `json:"merge_8,omitempty"`
	Field9  interface{} `json:"merge_9,omitempty"`
	Field10 interface{} `json:"merge_10,omitempty"`
	Field11 interface{} `json:"merge_11,omitempty"`
	Field12 interface{} `json:"merge_12,omitempty"`
	Field13 interface{} `json:"merge_13,omitempty"`
	Field14 interface{} `json:"merge_14,omitempty"`
	Field15 interface{} `json:"merge_15,omitempty"`
	Field16 interface{} `json:"merge_16,omitempty"`
	Field17 interface{} `json:"merge_17,omitempty"`
	Field18 interface{} `json:"merge_18,omitempty"`
	Field19 interface{} `json:"merge_19,omitempty"`
	Field20 interface{} `json:"merge_20,omitempty"`
	Field21 interface{} `json:"merge_21,omitempty"`
	Field22 interface{} `json:"merge_22,omitempty"`
	Field23 interface{} `json:"merge_23,omitempty"`
	Field24 interface{} `json:"merge_24,omitempty"`
	Field25 interface{} `json:"merge_25,omitempty"`
	Field26 interface{} `json:"merge_26,omitempty"`
	Field27 interface{} `json:"merge_27,omitempty"`
	Field28 interface{} `json:"merge_28,omitempty"`
	Field29 interface{} `json:"merge_29,omitempty"`
	Field30 interface{} `json:"merge_30,omitempty"`
	Field31 interface{} `json:"merge_31,omitempty"`
	Field32 interface{} `json:"merge_32,omitempty"`
	Field33 interface{} `json:"merge_33,omitempty"`
	Field34 interface{} `json:"merge_34,omitempty"`
	Field35 interface{} `json:"merge_35,omitempty"`
}

type DashaMailResponse struct {
	Response DashaMailResponseStruct `json:"response"`
}

type DashaMailResponseStruct struct {
	Msg  DashaMailResponseMsg  `json:"msg"`
	Data DashaMailResponseData `json:"data"`
}

type DashaMailResponseData []map[string]interface{}

type DashaMailResponseMsg struct {
	ErrorCode int64  `json:"err_code"`
	Text      string `json:"text"`
	Type      string `json:"type"`
}

type DashaMailUpdateInfo struct {
	WindowsShowed    interface{} `mapstructure:"окон_показано,omitempty"`
	WindowsConfirmed interface{} `mapstructure:"окон_подтверждено,omitempty"`
	MinutesOnline    interface{} `mapstructure:"просмотрено_минут_в_эфире,omitempty"`
	MinutesOffline   interface{} `mapstructure:"просмотрено_минут_в_записи,omitempty"`
	EventCodeName    interface{} `mapstructure:"кодировка_мероприятия,omitempty"`
	PointsZOView     interface{} `mapstructure:"бонусы_зо_за_просмотр,omitempty"`
	PointsZOQuestion interface{} `mapstructure:"бонусы_зо_за_вопрос,omitempty"`
	PointsZOPool     interface{} `mapstructure:"бонусы_зо_за_опрос,omitempty"`
	ViewRegime       interface{} `mapstructure:"режим_просмотра,omitempty"`

	Name                interface{} `mapstructure:"name,omitempty"`
	Phone               interface{} `mapstructure:"phone,omitempty"`
	Citizenship         interface{} `mapstructure:"citizenship,omitempty"`
	District            interface{} `mapstructure:"district,omitempty"`
	Region              interface{} `mapstructure:"region,omitempty"`
	City                interface{} `mapstructure:"city,omitempty"`
	Specialization      interface{} `mapstructure:"specialization,omitempty"`
	SpecializationExtra interface{} `mapstructure:"specializationExtra,omitempty"`
	WorkPlace           interface{} `mapstructure:"workPlace,omitempty"`
	Position            interface{} `mapstructure:"position,omitempty"`

	EventName      interface{} `mapstructure:"eventName,omitempty"`
	EventDate      interface{} `mapstructure:"eventDate,omitempty"`
	EventFormat    interface{} `mapstructure:"eventFormat,omitempty"`
	VisitationType interface{} `mapstructure:"visitationType,omitempty"`

	SourceUTM   interface{} `mapstructure:"sourceUTM,omitempty"`
	MediumUTM   interface{} `mapstructure:"mediumUTM,omitempty"`
	ContentUTM  interface{} `mapstructure:"contentUTM,omitempty"`
	CampaignUTM interface{} `mapstructure:"campaignUTM,omitempty"`

	Link interface{} `mapstructure:"link,omitempty"`
}

type YaDiskLoadedFileInfo struct {
	Link  string `json:"link,omitempty"`
	Error error  `json:"error,omitempty"`
}

type FacecastErrorResponse struct {
	Error string `json:"error"`
}

type FacecastLoginServerResponse struct {
	Key             string `json:"key,omitempty"`
	PersonalPhrases string `json:"personalPhrases,omitempty"`
}

type UserInfo struct {
	Message              string `json:"message"`
	Name                 string `json:"fio,omitempty"`
	Email                string `json:"email,omitempty"`
	Key                  string `json:"key,omitempty"`
	MinutesViewedOnline  int    `json:"minutesViewedOnline,omitempty"`
	FirstMinuteOnline    int    `json:"firstMinuteOnline"`
	LastMinuteOnline     int    `json:"lastMinuteOnline"`
	MinutesOnline        []int  `json:"minutesOnline,omitempty"`
	MinutesViewedOffline int    `json:"minutesViewedOffline,omitempty"`
	FirstMinuteOffline   int    `json:"firstMinuteOffline"`
	LastMinuteOffline    int    `json:"lastMinuteOffline"`
	MinutesOffline       []int  `json:"minutesOffline,omitempty"`
	Windows              int    `json:"confirmedWindows,omitempty"`
	AllWindows           int    `json:"allWindows,omitempty"`
	WayToAdd             string `json:"wayToAdd,omitempty"`
	EventID              string `json:"eventID,omitempty"`
	Citizenship          string `json:"citizenship,omitempty"`
	District             string `json:"district,omitempty"`
	Region               string `json:"region,omitempty"`
	City                 string `json:"city,omitempty"`
	Specialization       string `json:"specialization,omitempty"`
	SpecializationExtra  string `json:"specializationExtra,omitempty"`
	Position             string `json:"position,omitempty"`
	Own                  string `json:"own,omitempty"`
	ViewRegime           string `json:"viewRegime,omitempty"`
	PointsZOView         int    `json:"pointsZOView"`
}

type FacecastTicketResponse struct {
	Name     string `json:"fio"`
	Email    string `json:"email"`
	TicketID int64  `json:"id"`
}

type FacecastKeyResponse struct {
	TicketID int64  `json:"ticket_id"`
	Key      string `json:"password"`
}

type FacecastMinutesResponse struct {
	Key     string           `json:"password"`
	Minutes []FacecastMinute `json:"histogram"`
}

type FacecastMinute struct {
	Position int           `json:"position"`
	Views    []interface{} `json:"views"`
}

type FacecastWindowsResponse struct {
	Confirmed int    `json:"confirmed"`
	Key       string `json:"password"`
}

type FacecastInsertKeyResponse struct {
	Success bool `json:"success"`
}

type GetUserPointsServerResponse struct {
	YearPoints     int              `json:"yearPoints"`
	QuarterPoints  int              `json:"quarterPoints"`
	UserPointsInfo []UserPointsInfo `json:"userInfo,omitempty"`
}

type UserPointsInfo struct {
	EventDate   string `json:"event_date,omitempty"`
	EventName   string `json:"event_name,omitempty"`
	NMO         string `json:"nmo,omitempty"`
	ZET         string `json:"zet,omitempty"`
	Certificate string `json:"certificate,omitempty"`

	PointsZOView     string `json:"points_zo_view,omitempty"`
	PointsZOQuestion string `json:"points_zo_question,omitempty"`
	PointsZOPoll     string `json:"points_zo_poll,omitempty"`
}

type GetCertificatesInfoServerResponse struct {
	EventName string                             `json:"eventName,omitempty"`
	EventDate string                             `json:"eventDate,omitempty"`
	UsersInfo map[string]CertificatePersonalInfo `json:"usersInfo,omitempty"`
}

type CertificatePersonalInfo struct {
	UserName      string `json:"userName,omitempty"`
	ZET           string `json:"zet,omitempty"`
	NMO           string `json:"NMO,omitempty"`
	AcademicHours string `json:"academicHours,omitempty"`
}

type GetUserServerResponse struct {
	Message             string `json:"message"`
	Status              int    `json:"status,omitempty"`
	StatusExplain       string `json:"statusExplain,omitempty"`
	Name                string `json:"name,omitempty"`
	Phone               string `json:"phone,omitempty"`
	Citizenship         string `json:"citizenship,omitempty"`
	District            string `json:"district,omitempty"`
	Region              string `json:"region,omitempty"`
	City                string `json:"city,omitempty"`
	Specialization      string `json:"specialization,omitempty"`
	SpecializationExtra string `json:"specializationExtra,omitempty"`
	WorkPlace           string `json:"workPlace,omitempty"`
	Position            string `json:"position,omitempty"`
	Own                 string `json:"own,omitempty"`

	EventDate      string `json:"event_date,omitempty"`
	EventName      string `json:"event_name,omitempty"`
	NMO            string `json:"nmo,omitempty"`
	ZET            string `json:"zet,omitempty"`
	Certificate    string `json:"certificate,omitempty"`
	VisitationType string `json:"visitationType,omitempty"`
	AcademicHours  string `json:"academicHours,omitempty"`

	PointsZOView     string `json:"points_zo_view,omitempty"`
	PointsZOQuestion string `json:"points_zo_question,omitempty"`
	PointsZOPoll     string `json:"points_zo_poll,omitempty"`
}

type GetReportServerResponse struct {
	ReportName string                    `json:"reportName"`
	EventInfo  FacecastEventInfoResponse `json:"eventInfo,omitempty"`
	UsersInfo  map[string]UserInfo       `json:"usersInfo,omitempty"`
}

type CampaignReport struct {
	Date string `json:"date"`
	Name string `json:"name"`

	TagUTM     string `json:"tagUTM,omitempty"`
	SourceUTM  string `json:"sourceUTM,omitempty"`
	MediumUTM  string `json:"mediumUTM,omitempty"`
	ContentUTM string `json:"contentUTM,omitempty"`
	TermUTM    string `json:"termUTM,omitempty"`

	Sent              int    `json:"sent,omitempty"`
	Clicked           int    `json:"clicked,omitempty"`
	Opened            int    `json:"opened,omitempty"`
	FirstSent         string `json:"firstSent,omitempty"`
	FirstOpen         string `json:"firstOpen,omitempty"`
	LastOpen          string `json:"lastOpen,omitempty"`
	FirstClick        string `json:"firstClick,omitempty"`
	LastClick         string `json:"lastClick,omitempty"`
	UniqueOpened      int    `json:"uniqueOpened,omitempty"`
	UniqueClicked     int    `json:"uniqueClicked,omitempty"`
	Unsubscribed      int    `json:"unsubscribed,omitempty"`
	SpamComplained    int    `json:"spamComplained,omitempty"`
	SpamBlocked       int    `json:"spamBlocked,omitempty"`
	SpamMarked        int    `json:"spamMarked,omitempty"`
	MailSystemBlocked int    `json:"mailSystemBlocked,omitempty"`
	Hard              int    `json:"hard,omitempty"`
	Soft              int    `json:"soft,omitempty"`
}

type DashaMailCampaignReport struct {
	Date string `mapstructure:"delivery_time"`
	Name string `mapstructure:"name"`
	ID   string `mapstructure:"id"`

	TagUTM     string `mapstructure:"analytics_tag"`
	SourceUTM  string `mapstructure:"analytics_source"`
	MediumUTM  string `mapstructure:"analytics_medium"`
	ContentUTM string `mapstructure:"analytics_content"`
	TermUTM    string `mapstructure:"analytics_term"`

	Sent              string `json:"sent"`
	Clicked           string `json:"clicked"`
	Opened            string `json:"opened"`
	FirstSent         string `json:"first_sent"`
	FirstOpen         string `json:"first_open"`
	LastOpen          string `json:"last_open"`
	FirstClick        string `json:"first_click"`
	LastClick         string `json:"last_click"`
	UniqueOpened      string `json:"unique_opened"`
	UniqueClicked     string `json:"unique_clicked"`
	Unsubscribed      string `json:"unsubscribed"`
	SpamComplained    string `json:"complained"`
	SpamBlocked       string `json:"spam_blocked"`
	SpamMarked        string `json:"spam"`
	MailSystemBlocked string `json:"blk"`
	Hard              string `json:"hard"`
	Soft              string `json:"soft"`
}

type FacecastEventInfoResponse struct {
	VideoName      string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	PlanStartDate  string `json:"date_plan_start,omitempty"`
	StartDate      string `json:"date_real_start,omitempty"`
	EndDate        string `json:"date_real_end,omitempty"`
	Duration       int    `json:"duration_total,omitempty"`
	ViewersMax     int    `json:"viewers_max,omitempty"`
	TimeViewersMax string `json:"time_viewers_max,omitempty"`
	ViewersTotal   int    `json:"viewers_total,omitempty"`
	ReportTime     string `json:"report_time,omitempty"`
}

type ReportWritingDMLogs struct {
	WritingDMLogs ReportWritingDMLogsData `json:"writingDMLogsData,omitempty"`
}

type ReportWritingDMLogsData struct {
	SheetName string            `json:"sheetName"`
	Logs      map[string]string `json:"writingDMLogs,omitempty"`
}

type SendDataResponse struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	InfoDM  interface{} `json:"infoDM,omitempty"`
}

type WebSocketMessageRequest struct {
	APIMethod string      `json:"apiMethod"`
	Data      interface{} `json:"data"`
}

type WebSocketWaiterResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type WebSocketWaiter struct {
	Chan     *websocket.Conn
	Done     chan struct{}
	Ticker   *time.Ticker
	Response *WebSocketWaiterResponse
}
