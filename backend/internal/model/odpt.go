package model

type LocalizedString struct {
	Ja string `json:"ja,omitempty"`
	En string `json:"en,omitempty"`
}

// =========================
// Train Status
// =========================

type TrainStatus struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date  string `json:"dc:date"`
	Valid string `json:"dct:valid"`

	SameAs string `json:"owl:sameAs"`

	Railway  string `json:"odpt:railway"`
	Operator string `json:"odpt:operator"`

	TimeOfOrigin string `json:"odpt:timeOfOrigin,omitempty"`

	TrainInformationText LocalizedString `json:"odpt:trainInformationText"`
}

// =========================
// Railway
// =========================

type Railway struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date  string `json:"dc:date"`
	Title string `json:"dc:title"`

	SameAs string `json:"owl:sameAs"`

	LineCode string `json:"odpt:lineCode"`
	Operator string `json:"odpt:operator"`

	RailwayTitle LocalizedString `json:"odpt:railwayTitle"`

	StationOrder []StationOrder `json:"odpt:stationOrder"`

	AscendingRailDirection  string `json:"odpt:ascendingRailDirection"`
	DescendingRailDirection string `json:"odpt:descendingRailDirection"`
}

type StationOrder struct {
	Index int `json:"odpt:index"`

	Station string `json:"odpt:station"`

	StationTitle LocalizedString `json:"odpt:stationTitle"`
}

// =========================
// Station
// =========================

type Station struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date  string `json:"dc:date"`
	Title string `json:"dc:title"`

	Latitude  float64 `json:"geo:lat"`
	Longitude float64 `json:"geo:long"`

	SameAs string `json:"owl:sameAs"`

	Railway  string `json:"odpt:railway"`
	Operator string `json:"odpt:operator"`

	StationCode string `json:"odpt:stationCode"`

	StationTitle LocalizedString `json:"odpt:stationTitle"`

	PassengerSurvey []string `json:"odpt:passengerSurvey"`

	StationTimetable []string `json:"odpt:stationTimetable"`
}

// =========================
// Train Location
// =========================

type TrainLocation struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date  string `json:"dc:date"`
	Valid string `json:"dct:valid"`

	SameAs string `json:"owl:sameAs"`

	Railway  string `json:"odpt:railway"`
	Operator string `json:"odpt:operator"`

	TrainType  string `json:"odpt:trainType"`
	TrainOwner string `json:"odpt:trainOwner,omitempty"`

	TrainNumber string `json:"odpt:trainNumber"`

	RailDirection string `json:"odpt:railDirection"`

	Delay int `json:"odpt:delay"`

	FromStation *string `json:"odpt:fromStation"`
	ToStation   *string `json:"odpt:toStation"`

	OriginStation      []string `json:"odpt:originStation"`
	DestinationStation []string `json:"odpt:destinationStation"`
}

// =========================
// Station Timetable
// =========================

type StationTimetable struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date   string `json:"dc:date"`
	Issued string `json:"dct:issued"`

	SameAs string `json:"owl:sameAs"`

	Railway       string `json:"odpt:railway"`
	Station       string `json:"odpt:station"`
	Calendar      string `json:"odpt:calendar"`
	Operator      string `json:"odpt:operator"`
	RailDirection string `json:"odpt:railDirection"`

	StationTimetableObject []StationTimetableEntry `json:"odpt:stationTimetableObject"`
}

type StationTimetableEntry struct {
	ArrivalTime   string `json:"odpt:arrivalTime,omitempty"`
	DepartureTime string `json:"odpt:departureTime,omitempty"`

	ArrivalPlatform   string `json:"odpt:arrivalPlatform,omitempty"`
	DeparturePlatform string `json:"odpt:departurePlatform,omitempty"`

	Train string `json:"odpt:train,omitempty"`

	TrainType string `json:"odpt:trainType,omitempty"`

	TrainNumber string `json:"odpt:trainNumber,omitempty"`

	DestinationStation []string `json:"odpt:destinationStation,omitempty"`

	IsOrigin bool `json:"odpt:isOrigin,omitempty"`
	IsLast   bool `json:"odpt:isLast,omitempty"`

	Note string `json:"odpt:note,omitempty"`
}

// =========================
// Train Timetable
// =========================

type TrainTimetable struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date   string `json:"dc:date"`
	Issued string `json:"dct:issued"`

	Train  string `json:"odpt:train"`
	SameAs string `json:"owl:sameAs"`

	Railway       string `json:"odpt:railway"`
	Calendar      string `json:"odpt:calendar"`
	Operator      string `json:"odpt:operator"`
	TrainType     string `json:"odpt:trainType"`
	TrainNumber   string `json:"odpt:trainNumber"`
	RailDirection string `json:"odpt:railDirection"`

	OriginStation      []string `json:"odpt:originStation"`
	DestinationStation []string `json:"odpt:destinationStation"`

	TrainTimetableObject []TrainTimetableEntry `json:"odpt:trainTimetableObject"`
}

type TrainTimetableEntry struct {
	ArrivalTime   string `json:"odpt:arrivalTime,omitempty"`
	DepartureTime string `json:"odpt:departureTime,omitempty"`

	ArrivalStation   string `json:"odpt:arrivalStation,omitempty"`
	DepartureStation string `json:"odpt:departureStation,omitempty"`

	ArrivalPlatform   string `json:"odpt:arrivalPlatform,omitempty"`
	DeparturePlatform string `json:"odpt:departurePlatform,omitempty"`

	PlatformNumber string `json:"odpt:platformNumber,omitempty"`

	IsOrigin bool `json:"odpt:isOrigin,omitempty"`
	IsLast   bool `json:"odpt:isLast,omitempty"`

	Note string `json:"odpt:note,omitempty"`
}

// =========================
// Railway Fare
// =========================

type RailwayFare struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date   string `json:"dc:date"`
	Issued string `json:"dct:issued"`

	SameAs string `json:"owl:sameAs"`

	Operator string `json:"odpt:operator"`

	FromStation string `json:"odpt:fromStation"`
	ToStation   string `json:"odpt:toStation"`

	IcCardFare      int `json:"odpt:icCardFare"`
	TicketFare      int `json:"odpt:ticketFare"`
	ChildIcCardFare int `json:"odpt:childIcCardFare"`
	ChildTicketFare int `json:"odpt:childTicketFare"`
}

// =========================
// Passenger Survey
// =========================

type PassengerSurvey struct {
	ID      string `json:"@id"`
	Type    string `json:"@type"`
	Context string `json:"@context"`

	Date string `json:"dc:date"`

	SameAs string `json:"owl:sameAs"`

	Railway []string `json:"odpt:railway"`
	Station []string `json:"odpt:station"`

	Operator string `json:"odpt:operator"`

	IncludeAlighting bool `json:"odpt:includeAlighting"`

	PassengerSurveyObject []PassengerSurveyEntry `json:"odpt:passengerSurveyObject"`
}

type PassengerSurveyEntry struct {
	SurveyYear int `json:"odpt:surveyYear"`

	PassengerJourneys int `json:"odpt:passengerJourneys"`
}

type TrainView struct {
	TrainNumber string `json:"trainNumber"`

	Railway string `json:"railway"`

	FromStation string `json:"fromStation"`
	ToStation   string `json:"toStation"`

	Delay int `json:"delay"`
}
