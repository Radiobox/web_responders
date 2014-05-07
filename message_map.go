package web_responders

import (
	"fmt"
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

func (mm MessageMap) log(severity string, messages ...interface{}) {
	prefix := strings.ToUpper(severity)+":"
	log.Print(append([]interface{}{prefix}, messages...)...)
}

func (mm MessageMap) joinMessages(messages ...interface{}) string {
	response := ""
	for _, message := range messages {
		if response != "" {
			response += " "
		}
		switch src := message.(type) {
		case fmt.Stringer:
			response += src.String()
		case error:
			response += src.Error()
		case string:
			response += src
		}
	}
	return response
}

func (mm MessageMap) addMessage(severity string, messages ...interface{}) {
	go mm.log(severity, messages...)
	mm[severity] = append(mm[severity].([]string), mm.joinMessages(messages...))
}

// AddErrorMessage adds an error message to the message map.
func (mm MessageMap) AddErrorMessage(messages ...interface{}) {
	mm.addMessage("err", messages...)
}

// Errors returns a slice of all the error messages that have been
// added to this message map.
func (mm MessageMap) Errors() []string {
	return mm["err"].([]string)
}

// AddWarningMessage adds a warning message to the message map.
func (mm MessageMap) AddWarningMessage(messages ...interface{}) {
	mm.addMessage("warn", messages...)
}

// Warnings returns a slice of all warning messages that have been
// added to this message map.
func (mm MessageMap) Warnings() []string {
	return mm["warn"].([]string)
}

// AddInfoMessage adds an info message to this message map.
func (mm MessageMap) AddInfoMessage(messages ...interface{}) {
	mm.addMessage("info", messages...)
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
func (mm MessageMap) SetInputMessage(input string, messages ...interface{}) {
	inputErrs := mm.InputMessages()
	inputErrs[input] = mm.joinMessages(messages...)
}

func (mm MessageMap) InputMessages() map[string]string {
	return mm["input"].(map[string]string)
}
