package web_responders

type InputValidator interface {
	ValidateInput(interface{}) error
}
