package entity

type TriggerTemplate struct {
	TemplateID  string `json:"template_id"`
	TriggerType string `json:"trigger_type"`
	Condition   string `json:"condition"`
	Body        string `json:"body"`
	Channel     string `json:"channel"`
}
