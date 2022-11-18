package api

import (
	"encoding/json"
	"fmt"
	"github.com/benoitmasson/plotters/piechart"
	"github.com/mitchellh/mapstructure"
	"github.com/wcharczuk/go-chart/v2/drawing"
	excel "github.com/xuri/excelize/v2"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func NewServerDebug(initialMethod string) *ServerDebug {
	return &ServerDebug{
		ExecutionStages:  initialMethod,
		LastResponseData: nil,
		LastSentData:     nil,
	}
}

func (sd *ServerDebug) SetDebugFinalStage(err *error, finalStage string) {
	/*/
	 * Во-первых, нет смысла перезаписывать, если err == nil, т.к. debug.Error по умолчанию nil, но лишний if - визуальная перегрузка.
	 * Во-вторых, естественно, нет необходимости заводить отдельную переменную для ошибки и возвращать ее из всех вложенных функций, т.к.
	 * в качестве аргумента передаю указатель на структуру, куда потом кладу ошибку. По идее, могу вписывать ошибку сразу по указателю, но:
	 * 1) долго исправлять;
	 * 2) кажется, что визуально нестандартно.
	 * Пока оставляю так, на досуге подумаю. Сейчас, в любом случае, из функций обработки логики API-запроса вместо трех параметров возвращаю
	 * два (два вложены в структуру ServerDebug, т.к. логически относятся к ошибке), что кажется визуально приятнее
	/*/
	sd.Error = *err
	if *err == nil {
		sd.SetDebugLastStage(finalStage)
	}
	sd.ExecutionStages = strings.TrimSuffix(sd.ExecutionStages, " -> ")
}

func (sd *ServerDebug) SetDebugLastStage(lastStage string) {
	st := strings.Split(sd.ExecutionStages, " -> ")
	st[len(st)-1] = lastStage
	sd.ExecutionStages = strings.Join(st, " -> ")
}

func (sd *ServerDebug) DeleteDebugLastStage(err *error) {
	if *err == nil {
		st := strings.Split(sd.ExecutionStages, " -> ")
		sd.ExecutionStages = strings.Join(st[:len(st)-1], " -> ")
	}
}

func (sd *ServerDebug) SetDebugData(lastSentData, lastResponseData *[]byte) {
	sd.LastSentData = *lastSentData
	sd.LastResponseData = *lastResponseData
}

func getErrorMessage(debug *ServerDebug) string {
	errorMessage := ""

	if debug.ExecutionStages != "" {
		errorMessage = fmt.Sprintf("es: %s", debug.ExecutionStages)
	}

	if debug.LastResponseData != nil {
		if errorMessage == "" {
			errorMessage = fmt.Sprintf("rd: %s", debug.LastResponseData)
		} else {
			errorMessage += fmt.Sprintf(", rd: %s", debug.LastResponseData)
		}
	}

	if debug.LastSentData != nil {
		if errorMessage == "" {
			errorMessage = fmt.Sprintf("sd: %s", debug.LastSentData)
		} else {
			errorMessage += fmt.Sprintf(", sd: %s", debug.LastSentData)
		}
	}

	if errorMessage == "" {
		errorMessage = debug.Error.Error()
	} else {
		errorMessage = fmt.Sprintf("%s {%s}", debug.Error.Error(), errorMessage)
	}

	return errorMessage
}

func GetEventMaxPoints(videoName string) int {
	switch videoName {
	case "Вебинар", "Вебинар НМО", "Интерактивная школа", "Интерактивная школа НМО":
		return 20
	case "Круглый стол", "Круглый стол НМО", "Медицинский тренинг", "Медицинский тренинг НМО":
		return 30
	case "Конференция межрегиональная", "Конференция межрегиональная НМО":
		return 40
	case "Конференция ЗО", "Конференция ЗО НМО", "Конференция НАСИБ", "Конференция НАСИБ НМО":
		return 50
	default:
		return 0
	}
}

func AppendMinutesIfMissing(slice *[]int, elems ...int) bool {
	changed := false
	defer func() {
		/*/
		 * Для экономии времени сортируем slice только при условии, что он изменился, т.к. точно знаем,
		 * что на вход подается уже отсортированный по возрастанию slice.
		/*/
		if changed {
			sort.Slice(*slice, func(i, j int) bool { return (*slice)[i] < (*slice)[j] })
		}
	}()

	for _, elem := range elems {
		needAppend := true
		for _, sliceElem := range *slice {
			if sliceElem == elem {
				needAppend = false
				break
			}
		}

		if needAppend {
			*slice = append(*slice, elem)
			changed = true
		}
	}

	return changed
}

func FilterMinutesOffline(minutesOffline *[]int, minutesOnline ...int) bool {
	if len(minutesOnline) == 0 {
		return false
	}

	newMinutesOffline := make([]int, 0)
	defer func() {
		if len(*minutesOffline) != len(newMinutesOffline) {
			*minutesOffline = newMinutesOffline // для экономии времени переписываем minutesOffline только при условии, что он изменился
		}
	}()

	/*/
	 * Можно было бы написать проще (без кучи доп. условий и break), но на этом этапе оба массива точно отсортированы
	 * по возрастанию. Это значит, что можно не проверять часть элементов (сокращение времени).
	/*/
	for _, minuteOffline := range *minutesOffline {
		for minuteOnlineNum, minuteOnline := range minutesOnline {
			if minuteOffline == minuteOnline {
				break
			} else {
				if minuteOffline < minuteOnline {
					newMinutesOffline = append(newMinutesOffline, minuteOffline)
					break
				}
			}

			if minuteOnlineNum == len(minutesOnline)-1 {
				newMinutesOffline = append(newMinutesOffline, minuteOffline)
			}
		}
	}

	/*/
	 * Т.к. из исходного слайса удаляются элементы, переданные для проверки, то признаком изменения
	 * можно выбрать изменение длины исходного слайса.
	/*/
	return len(*minutesOffline) != len(newMinutesOffline)
}

func GetViewRegime(minutesViewedOnline, minutesViewedOffline int) string {
	switch {
	case minutesViewedOnline > 0 && minutesViewedOffline > 0:
		return "прямой эфир и запись"
	case minutesViewedOnline > 0 && minutesViewedOffline == 0:
		return "прямой эфир"
	case minutesViewedOnline == 0 && minutesViewedOffline > 0:
		return "запись"
	default:
		return ""
	}
}

func GetPointsZOView(eventMaxPoints, duration, minutesViewed int) float64 {
	if minPart := float64(minutesViewed) / float64(duration); minPart < 0.1 {
		return 0
	} else {
		return math.Round(minPart * float64(eventMaxPoints))
	}
}

func WriteExcelWebinarData(f *excel.File, reportData []interface{}, debug *ServerDebug) (map[string]int, map[string]int, map[int]struct{ Online, Offline, Total int }, error) {
	debug.SetDebugLastStage("WriteExcelWebinarData")

	G2 := int(reportData[1].([]interface{})[6].(float64)) // клетка G2 - продолжительность эфира
	viewingRegimes, specialisations, minutesDistribution := GetPlottingVariables(G2)
	for rowNum, row := range reportData {
		line := row.([]interface{})
		if rowNum > 2 { // пропускаем заголовок excel-отчета
			AddViewerRegime(viewingRegimes, line[21].(string))                             // столбец V - режим просмотра
			AddViewerSpecialisation(specialisations, line[19].(string), line[21].(string)) // столбец T - специализация и столбец V - режим просмотра
			minutesOnline, _ := line[10].([]interface{})                                   // столбец K - просмотренные минуты онлайн
			minutesOffline, _ := line[14].([]interface{})                                  // столбец O - просмотренные минуты офлайн
			AddViewerMinutes(minutesDistribution, minutesOnline, minutesOffline)
		}

		err := f.SetSheetRow("Отчёт", "A"+strconv.Itoa(rowNum+1), &line)
		if err != nil {
			err = fmt.Errorf("%+v (rowNum: %v row: %+v)", err, rowNum, row)
			return nil, nil, nil, err
		}
	}

	return *viewingRegimes, *specialisations, *minutesDistribution, nil
}

func GetPlottingVariables(eventDuration int) (*map[string]int, *map[string]int, *map[int]struct{ Online, Offline, Total int }) {
	viewingRegimes := map[string]int{
		"в эфире":            0,
		"в записи":           0,
		"в эфире и в записи": 0,
	}

	specialisations := make(map[string]int)

	minutesDistribution := make(map[int]struct{ Online, Offline, Total int })
	for i := 0; i < eventDuration; i++ {
		minutesDistribution[i+1] = struct {
			Online  int
			Offline int
			Total   int
		}{}
	}

	return &viewingRegimes, &specialisations, &minutesDistribution
}

func AddViewerRegime(plotData *map[string]int, regime string) {
	switch regime {
	case "прямой эфир":
		(*plotData)["в эфире"]++
	case "запись":
		(*plotData)["в записи"]++
	case "прямой эфир и запись":
		(*plotData)["в эфире и в записи"]++
	}
}

func AddViewerSpecialisation(plotData *map[string]int, specialisation, regime string) {
	if specialisation != "" && regime != "" {
		(*plotData)[specialisation]++
	}
}

func AddViewerMinutes(plotData *map[int]struct{ Online, Offline, Total int }, minutesOnline, minutesOffline []interface{}) {
	for _, _minute := range minutesOnline {
		minute := int(_minute.(float64))
		(*plotData)[minute] = struct{ Online, Offline, Total int }{
			Online:  (*plotData)[minute].Online + 1,
			Offline: (*plotData)[minute].Offline,
			Total:   (*plotData)[minute].Total + 1,
		}
	}

	for _, _minute := range minutesOffline {
		minute := int(_minute.(float64))
		(*plotData)[minute] = struct{ Online, Offline, Total int }{
			Online:  (*plotData)[minute].Online,
			Offline: (*plotData)[minute].Offline + 1,
			Total:   (*plotData)[minute].Total + 1,
		}
	}
}

func PlotWebinarCharts(f *excel.File, viewingRegimesChartData, specialisationsChartData map[string]int, minutesDistributionChartData map[int]struct{ Online, Offline, Total int }, debug *ServerDebug) error {
	debug.SetDebugLastStage("PlotWebinarCharts -> ")

	var err error
	defer debug.DeleteDebugLastStage(&err)

	const (
		VIEWING_REGIMES_CHART      = "viewingRegimes.jpeg"
		SPECIALISATIONS_CHART      = "specialisations.jpeg"
		MINUTES_DISTRIBUTION_CHART = "minutesDistribution.jpeg"
	)

	f.NewSheet("Графики")
	if err = PlotPieChart(f, VIEWING_REGIMES_CHART, viewingRegimesChartData, debug); err != nil {
		return err
	}

	if err = PlotPieChart(f, SPECIALISATIONS_CHART, specialisationsChartData, debug); err != nil {
		return err
	}

	if err = PlotAreaChart(f, MINUTES_DISTRIBUTION_CHART, minutesDistributionChartData, debug); err != nil {
		return err
	}

	return nil
}

func PlotPieChart(f *excel.File, plotName string, plotData map[string]int, debug *ServerDebug) error {
	debug.SetDebugLastStage("PlotPieChart")

	var piePartsData []struct {
		Label          string
		Value, Percent float64
	}
	p, piePartsJSONData, generalPieParams, plotCell := InitPieChart(plotName, plotData)
	_ = json.Unmarshal(piePartsJSONData, &piePartsData)

	for colorRGBANum, piePartData := range piePartsData {
		pie := GetPiePart(colorRGBANum, piePartData, &generalPieParams)
		p.Legend.Add(piePartData.Label, pie)
		p.Add(pie)
	}

	if err := p.Save(15*vg.Centimeter, 20*vg.Centimeter, plotName); err != nil {
		err = fmt.Errorf("error with chart %s: %+v", plotName, err)
		return err
	}

	if err := f.AddPicture("Графики", plotCell, plotName, `{"x_scale": 0.565,"y_scale": 0.565}`); err != nil {
		err = fmt.Errorf("can't add chart %s to excel report: %+v", plotName, err)
		return err
	}

	if err := os.Remove(plotName); err != nil {
		err = fmt.Errorf("can't remove chart %s after importing to excel report: %+v", plotName, err)
		return err
	}

	return nil
}

func InitPieChart(plotName string, plotData map[string]int) (*plot.Plot, []byte, struct{ Total, OffsetValue float64 }, string) {
	p := plot.New()
	p.HideAxes()
	p.Legend.Top = true
	p.Legend.Left = true
	p.Legend.TextStyle.Font.Size = 5 * vg.Millimeter // размер шрифта подписей на графике
	p.Legend.Padding = 2 * vg.Millimeter             // расстрояние между строками в легенде
	p.Title.TextStyle.Font.Size = 8 * vg.Millimeter  // размер шрифта заголовка
	p.Title.Padding = 8 * vg.Millimeter              // расстрояние между заголовком и легендой?
	plotCell := ""

	switch {
	case strings.Contains(plotName, "viewingRegimes"):
		p.Title.Text = "\nСтатистика просмотров" // не нашел в настройках отступы от верхнего края, поэтому доп. строка как отступ
		plotCell = "A1"
	case strings.Contains(plotName, "specialisations"):
		p.Title.Text = "\nСпециализации" // не нашел в настройках отступы от верхнего края, поэтому доп. строка как отступ
		plotCell = "G1"
	}

	type PiePartData struct {
		Label          string
		Value, Percent float64
	}
	piePartsData := make([]PiePartData, 0)
	total := 0.

	for label, value := range plotData {
		total += float64(value)
		piePartsData = append(piePartsData, PiePartData{
			Label: label,
			Value: float64(value),
		})
	}

	sort.Slice(piePartsData, func(i, j int) bool {
		value_i := piePartsData[i].Value
		value_j := piePartsData[j].Value
		piePartsData[i].Percent = math.Round(100*value_i/total*100) / 100
		piePartsData[j].Percent = math.Round(100*value_j/total*100) / 100

		return value_i > value_j
	})

	MAX_PIE_PARTS := 5
	if len(piePartsData) > MAX_PIE_PARTS {
		others := PiePartData{Label: "Другие"}
		for _, minorElement := range piePartsData[MAX_PIE_PARTS:] {
			others.Value += minorElement.Value
			others.Percent = math.Round(100*float64(others.Value)/total*100) / 100
		}

		piePartsData[MAX_PIE_PARTS] = others
		piePartsData = piePartsData[:MAX_PIE_PARTS+1]
	}
	piePartsJSONData, _ := json.Marshal(piePartsData)

	return p, piePartsJSONData, struct{ Total, OffsetValue float64 }{Total: total}, plotCell
}

func GetPiePart(colorRGBANum int, piePartData struct {
	Label          string
	Value, Percent float64
}, generalPieParams *struct{ Total, OffsetValue float64 }) *piechart.PieChart {
	defer func() { generalPieParams.OffsetValue += piePartData.Value }()

	colorsRGBA := []color.RGBA{{255, 255, 0, 255} /*желтый*/, {255, 192, 203, 255} /*розовый*/, {255, 165, 0, 255} /*рыжий*/, {154, 206, 235, 255} /*голубой*/, {144, 238, 144, 255} /*зеленый*/, {100, 149, 237, 255} /*синий*/}
	pie, _ := piechart.NewPieChart(plotter.Values{piePartData.Value})
	pie.Total = generalPieParams.Total
	pie.Color = colorsRGBA[colorRGBANum]
	pie.Labels.Position = 0.7
	pie.Labels.TextStyle.Rotation = GetTextStyleRotation(piePartData.Value, generalPieParams.OffsetValue, generalPieParams.Total)
	pie.Labels.Nominal = []string{fmt.Sprintf("%v (~%v%s)", piePartData.Value, piePartData.Percent, "%")}
	pie.Offset.Value = generalPieParams.OffsetValue
	pie.Offset.Y = -15 * vg.Millimeter
	// pie.Labels.Values.Show = false // они по умолчанию false, но оставил здесь в комментарии, чтобы, если понадобится, не искать в доках
	// pie.Labels.Values.Percentage = false

	return pie
}

func GetTextStyleRotation(value, offsetValue, total float64) float64 {
	if value/total > 0.1 {
		return 0. // в целом, для визуального удобства (если доля очередного сектора ПРИМЕРНО не меньше 10%, то надпись поместится и горизонтально)
	}

	angle := 2 * math.Pi * (offsetValue + value/2) / total // отсчет угла ведется из нижнего края первой четверти координатной плоскости (справа) против часовой стрелки
	switch {
	case math.Pi/2 < angle && angle < math.Pi*3/2:
		return angle + math.Pi
	default:
		return angle
	}
}

func PlotAreaChart(f *excel.File, plotName string, plotData map[int]struct{ Online, Offline, Total int }, debug *ServerDebug) error {
	debug.SetDebugLastStage("PlotAreaChart")

	p, areaChartSeries := InitAreaChart(plotData)
	for colorRGBANum, series := range areaChartSeries {
		line, err := GetAreaChartLine(colorRGBANum, series.Values)
		if err != nil {
			return err
		}
		p.Legend.Add(series.Label, line)
		p.Add(line)
	}

	if err := p.Save(20*vg.Centimeter, 15*vg.Centimeter, plotName); err != nil {
		return err
	}

	if err := f.AddPicture("Графики", "M1", plotName, `{"x_scale": 0.7,"y_scale": 0.7}`); err != nil {
		err = fmt.Errorf("can't add chart %s to excel report: %+v", plotName, err)
		return err
	}

	if err := os.Remove(plotName); err != nil {
		err = fmt.Errorf("can't remove chart %s after importing to excel report: %+v", plotName, err)
		return err
	}

	return nil
}

func InitAreaChart(plotData map[int]struct{ Online, Offline, Total int }) (*plot.Plot, []struct {
	Label  string
	Values plotter.XYs
}) {
	p := plot.New()
	p.Legend.Top = true
	p.Legend.TextStyle.Font.Size = 4 * vg.Millimeter // размер шрифта подписей на графике
	p.Legend.Padding = 2 * vg.Millimeter             // расстрояние между строками в легенде
	p.Title.TextStyle.Font.Size = 8 * vg.Millimeter  // размер шрифта заголовка
	p.Title.Padding = 8 * vg.Millimeter              // расстрояние между заголовком и легендой?
	p.Title.Text = "\nТайминг"                       // не нашел в настройках отступы от верхнего края, поэтому доп. строка как отступ
	p.X.Label.Text = "минуты\n "                     // не нашел в настройках отступы от нижнего края, поэтому доп. строка как отступ
	p.X.Label.TextStyle.Font.Size = 5 * vg.Millimeter
	p.Y.Label.Text = "\nчисло зрителей" // не нашел в настройках отступы от левого края, поэтому доп. строка как отступ
	p.Y.Label.TextStyle.Font.Size = 5 * vg.Millimeter
	p.Add(plotter.NewGrid())

	var (
		onlineDistribution  = make(plotter.XYs, 0)
		offlineDistribution = make(plotter.XYs, 0)
		totalDistribution   = make(plotter.XYs, 0)
	)
	type AreasData struct {
		Minute       int
		Distribution struct{ Online, Offline, Total int }
	}
	areasData := make([]AreasData, 0)

	for minute, distribution := range plotData {
		areasData = append(areasData, AreasData{
			Minute:       minute,
			Distribution: distribution,
		})
	}

	sort.Slice(areasData, func(i, j int) bool {
		return areasData[i].Minute < areasData[j].Minute
	})

	for _, data := range areasData {
		totalDistribution = append(totalDistribution, plotter.XY{X: float64(data.Minute), Y: float64(data.Distribution.Total)})
		onlineDistribution = append(onlineDistribution, plotter.XY{X: float64(data.Minute), Y: float64(data.Distribution.Online)})
		offlineDistribution = append(offlineDistribution, plotter.XY{X: float64(data.Minute), Y: float64(data.Distribution.Offline)})
	}

	return p, []struct {
		Label  string
		Values plotter.XYs
	}{
		{Label: "в эфире", Values: onlineDistribution},
		{Label: "в записи", Values: offlineDistribution},
		{Label: "в эфире и в записи", Values: totalDistribution},
	}
}

func GetAreaChartLine(colorRGBANum int, series plotter.XYs) (*plotter.Line, error) {
	colorsRGBA := []struct{ lineColor, fillColor drawing.Color }{
		{lineColor: drawing.Color{R: 255, G: 255, B: 0, A: 255}, fillColor: drawing.Color{R: 255, G: 255, B: 0, A: 60}},     /*желтый*/
		{lineColor: drawing.Color{R: 144, G: 238, B: 144, A: 255}, fillColor: drawing.Color{R: 144, G: 238, B: 144, A: 50}}, /*зеленый*/
		{lineColor: drawing.Color{R: 100, G: 149, B: 237, A: 255}, fillColor: drawing.Color{R: 100, G: 149, B: 237, A: 40}}, /*синий*/
	}
	line, err := plotter.NewLine(series)
	if err != nil {
		return nil, err
	}
	line.LineStyle.Width = 0.5 * vg.Millimeter
	line.LineStyle.Color = colorsRGBA[colorRGBANum].lineColor
	line.FillColor = colorsRGBA[colorRGBANum].fillColor

	return line, nil
}

func WriteExcelCampaignsData(f *excel.File, reportData []interface{}, debug *ServerDebug) error {
	debug.SetDebugLastStage("WriteExcelCampaignsData")

	for rowNum, row := range reportData {
		line := row.([]interface{})
		err := f.SetSheetRow("Отчёт", "A"+strconv.Itoa(rowNum+1), &line)
		if err != nil {
			err = fmt.Errorf("%+v (rowNum: %v row: %+v)", err, rowNum, row)
			return err
		}
	}

	return nil
}

// AutoResizeColumns sets the width of the columns according to their text content and fixed height.
func AutoResizeColumns(f *excel.File, sheetName string, debug *ServerDebug) error {
	debug.SetDebugLastStage("AutoResizeColumns")

	cols, err := f.GetCols(sheetName)
	if err != nil {
		return err
	}

	for colNum, col := range cols {
		alignmentStyle := &excel.Style{
			Alignment: &excel.Alignment{
				Horizontal: "left",
				Vertical:   "center",
			},
		}

		alignmentStyleID, err := f.NewStyle(alignmentStyle)
		if err != nil {
			return err
		}

		colName, err := excel.ColumnNumberToName(colNum + 1)
		if err != nil {
			return err
		}

		err = f.SetColStyle(sheetName, colName, alignmentStyleID)
		if err != nil {
			return err
		}

		largestWidth := 0
		for rowNum, rowCell := range col {
			cellWidth := utf8.RuneCountInString(rowCell) + 5 // + 5 for margin
			if cellWidth > largestWidth {
				largestWidth = cellWidth
			}

			err := f.SetRowHeight(sheetName, rowNum+1, 15)
			if err != nil {
				return err
			}
		}

		if largestWidth > 150 {
			largestWidth = 150 //utf8.RuneCountInString(col[refRowNum]) + 5 // + 5 for margin; col[refRowNum] - клетка, относительно которой выставляется ширина всего столбца, если максимальная ширина клетки в столбце превышает 150
		}

		err = f.SetColWidth(sheetName, colName, colName, float64(largestWidth))
		if err != nil {
			return err
		}
	}

	return nil
}

func ISOToHuman(iso string) string {
	t, _ := time.Parse(time.RFC3339, iso)

	return TimeToHuman(t)
}

func TimeToHuman(t time.Time) string {
	//location, err := time.LoadLocation("Europe/Moscow")
	location := time.FixedZone("UTC+3", 3*60*60) // ?
	millisec := t.In(location).UnixNano() / 1e6

	return time.Unix(0, millisec*int64(time.Millisecond)).In(location).Format("02.01.2006 15:04")
}

func LogWebSocketError(err error) {
	if err != nil && UnsafeError(err) {
		fmt.Println(fmt.Errorf("+++++++ CAN'T SEND AN ERROR TO FRONTEND: %+v +++++++", err))
	}
}

//func GenerateKey(passwordLength int) string {
//	var (
//		password 	   strings.Builder
//		minNum 		   = 1
//		minUpperCase   = 1
//
//		lowerCharSet   = "abcdedfghijklmnopqrstuvwxyz"
//		upperCharSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
//		numberSet      = "0123456789"
//		allCharSet     = lowerCharSet + upperCharSet + numberSet
//	)
//
//	// Set numeric
//	for i := 0; i < minNum; i++ {
//		random := rand.Intn(len(numberSet))
//		password.WriteString(string(numberSet[random]))
//	}
//
//	// Set uppercase
//	for i := 0; i < minUpperCase; i++ {
//		random := rand.Intn(len(upperCharSet))
//		password.WriteString(string(upperCharSet[random]))
//	}
//
//	remainingLength := passwordLength - minNum - minUpperCase
//	for i := 0; i < remainingLength; i++ {
//		random := rand.Intn(len(allCharSet))
//		password.WriteString(string(allCharSet[random]))
//	}
//	inRune := []rune(password.String())
//	rand.Shuffle(len(inRune), func(i, j int) {
//		inRune[i], inRune[j] = inRune[j], inRune[i]
//	})
//	pswd := string(inRune)
//
//	for _, user := range *USERS {
//		if user.Key == pswd {
//			return generateKey(16)
//		}
//	}
//
//	return pswd
//}

func UnmarshalResponseData(response []byte) DashaMailResponseStruct {
	var data DashaMailResponse
	_ = json.Unmarshal(response, &data)

	return data.Response
}

func DecodeToStruct(i interface{}, data map[string]interface{}, debug *ServerDebug) (interface{}, error) {
	debug.SetDebugLastStage("DecodeToStruct")

	t := reflect.TypeOf(i).Elem()
	result := reflect.New(t).Interface()

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &result,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(data)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d DashaMailResponseData) CheckForOnlyOneElement(debug *ServerDebug) error {
	debug.SetDebugLastStage("CheckForOnlyOneElement")

	if len(d) == 0 {
		return fmt.Errorf("DashaMail answered without an error but with nil data array")
	}

	if len(d) > 1 {
		return fmt.Errorf("DashaMail answered with %v elements in data array but only one element is expected (%+v)", len(d), d)
	}

	return nil
}

func (m DashaMailResponseMsg) CheckForError(debug *ServerDebug) error {
	debug.SetDebugLastStage("CheckForError")

	meaning := ""
	switch m.ErrorCode {
	case 0:
		return nil
	case 1:
		meaning = "неверный логин и(или) пароль"
	case 2:
		meaning = "ошибка при добавлении в базу"
	case 3:
		meaning = "заданы не все необходимые параметры"
	case 4:
		meaning = "нет данных при выводе"
	case 5:
		meaning = "у пользователя нет адресной базы с таким id"
	case 6:
		meaning = "некорректный email-адрес"
	case 7:
		meaning = "такой пользователь уже есть в этой адресной базе"
	case 8:
		meaning = "лимит по количеству активных подписчиков на тарифном плане клиента"
	case 9:
		meaning = "нет такого подписчика у клиента"
	case 10:
		meaning = "пользователь уже отписан"
	case 11:
		meaning = "нет данных для обновления подписчика"
	case 12:
		meaning = "не заданы элементы списка"
	case 13:
		meaning = "не задано время рассылки"
	case 14:
		meaning = "не задан заголовок письма"
	case 15:
		meaning = "не задано поле 'От Кого?'"
	case 16:
		meaning = "не задан обратный адрес"
	case 17:
		meaning = "не задана ни html, ни plain_text версия письма"
	case 18:
		meaning = "нет ссылки отписаться (ссылки с id='unsub_link') в тексте рассылки"
	case 19:
		meaning = "нет ссылки отписаться ('%ОТПИСАТЬСЯ%') в тексте рассылки"
	case 20:
		meaning = "задан недопустимый статус рассылки"
	case 21:
		meaning = "рассылка уже отправляется"
	case 22:
		meaning = "у вас нет кампании с таким campaign_id"
	case 23:
		meaning = "нет такого поля для сортировки"
	case 24:
		meaning = "заданы недопустимые события для авторассылки"
	case 25:
		meaning = "загружаемый файл уже существует"
	case 26:
		meaning = "загружаемый файл больше 5 Мб"
	case 27:
		meaning = "файл не найден"
	case 28:
		meaning = "указанный шаблон не существует"
	case 29:
		meaning = "определен одноразовый email-адрес"
	case 30:
		meaning = "отправка рассылок заблокирована по подозрению в спаме"
	case 31:
		meaning = "массив email-адресов пуст"
	case 32:
		meaning = "нет корректных адресов для добавления"
	case 33:
		meaning = "недопустимый формат файла"
	case 34:
		meaning = "необходимо настроить собственный домен отправки"
	case 35:
		meaning = "данный функционал недоступен на бесплатных тарифах и во время триального периода"
	case 36:
		meaning = "ошибка при отправке письма"
	case 37:
		meaning = "рассылка еще не прошла модерацию"
	case 38:
		meaning = "недопустимый сегмент"
	case 39:
		meaning = "нет папки с таким id"
	case 40:
		meaning = "рассылка не находится в статусе 'PROCESSING' или 'SENT'"
	case 41:
		meaning = "рассылка не отправляется в данный момент"
	case 42:
		meaning = "у вас нет рассылки на паузе с таким campaign_id"
	case 43:
		meaning = "пользователь в черном списке (двойная отписка)"
	case 44:
		meaning = "пользователь в черном списке (нажатие 'ЭТО СПАМ')"
	case 45:
		meaning = "пользователь в черном списке (ручное)"
	case 46:
		meaning = "несуществующий email-адрес (находится в глобальном списке возвратов)"
	case 47:
		meaning = "ваш IP-адрес не включен в список разрешенных"
	case 48:
		meaning = "не удалось отправить письмо подтверждения для обратного адреса"
	case 49:
		meaning = "такой адрес уже подтвержден"
	case 50:
		meaning = "нельзя использовать одноразовые email в обратном адресе"
	case 51:
		meaning = "использование обратного адреса на публичных доменах Mail.ru СТРОГО ЗАПРЕЩЕНО политикой DMARC данного почтового провайдера"
	case 52:
		meaning = "email-адрес не подтвержден в качестве отправителя"
	case 53:
		meaning = "недопустимое событие для webhook"
	case 54:
		meaning = "некорректный домен, т.к. кириллические и другие национальные домены в качестве DKIM/SPF запрещены"
	case 55:
		meaning = "данный домен находится в черном списке, его добавление запрещено"
	case 56:
		meaning = "данный домен занят другим аккаунтом"
	default:
		return fmt.Errorf("DashaMail unknown error: %+v", m)
	}

	return fmt.Errorf("DashaMail error with code %v: %s {meaning %s}", m.ErrorCode, m.Text, meaning)
}
