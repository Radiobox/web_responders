package web_responders

import (
	"log"
	"strings"
)

// MessageMap is a map intended to be used for carrying messages
// around, for the purpose of error handling.  It will also
// (concurrently) log messages using the logging package.  Methods on
// MessageMap always expect the MessageMap to already contain the keys
// "err", "warn", and "info"; and for each of those to contain a slice
// of strings.  You can use NewMessageMap() to set up an empty
// MessageMap value.
type MessageMap map[string]interface{}

// NewMessageMap returns a MessageMap that is properly initialized.
func NewMessageMap() MessageMap {
	return MessageMap{
		"err":   []string{},
		"warn":  []string{},
		"info":  []string{},
		"input": map[string]string{},
	}
}

func (mm MessageMap) log(severity, message string) {
	log.Print(strings.ToUpper(severity) + ": " + message)
}

func (mm MessageMap) addMessage(severity, message string) {
	go mm.log(severity, message)
	mm[severity] = append(mm[severity].([]string), message)
}

// AddErrorMessage adds an error message to the message map.
func (mm MessageMap) AddErrorMessage(message string) {
	mm.addMessage("err", message)
}

// Errors returns a slice of all the error messages that have been
// added to this message map.
func (mm MessageMap) Errors() []string {
	return mm["err"].([]string)
}

// AddWarningMessage adds a warning message to the message map.
func (mm MessageMap) AddWarningMessage(message string) {
	mm.addMessage("warn", message)
}

// Warnings returns a slice of all warning messages that have been
// added to this message map.
func (mm MessageMap) Warnings() []string {
	return mm["warn"].([]string)
}

// AddInfoMessage adds an info message to this message map.
func (mm MessageMap) AddInfoMessage(message string) {
	mm.addMessage("info", message)
}

// Infos returns a slice of all info messages that have been added to
// this message map.
func (mm MessageMap) Infos() []string {
	return mm["info"].([]string)
}

// NumErrors is sugar for len(MessageMap.Errors())
func (mm MessageMap) NumErrors() int {
	return len(mm.Errors())
}

// NumErrors is sugar for len(MessageMap.Warnings())
func (mm MessageMap) NumWarnings() int {
	return len(mm.Warnings())
}

// NumErrors is sugar for len(MessageMap.Infos())
func (mm MessageMap) NumInfos() int {
	return len(mm.Infos())
}

// SetInputError adds an error message for a specific input name.
func (mm MessageMap) SetInputMessage(input, message string) {
	inputErrs := mm.InputMessages()
	inputErrs[input] = message
}

func (mm MessageMap) InputMessages() map[string]string {
	return mm["input"].(map[string]string)
}
