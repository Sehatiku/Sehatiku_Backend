package model

type ScheduleEntry struct {
	Days string `json:"days"`
	Time string `json:"time"`
}

type AssignedNakesResponse struct {
	FullName       string          `json:"full_name"`
	Specialization string          `json:"specialization"`
	Hospital       string          `json:"hospital"`
	WhatsappPhone  string          `json:"whatsapp_phone"`
	WaLink         string          `json:"wa_link"`
	Schedule       []ScheduleEntry `json:"schedule"`
}
