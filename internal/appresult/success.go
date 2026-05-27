package appresult

import "encoding/json"

var (
	Success         = NewAppSuccess("Success !!!", "SS-10000", nil)
	SuccessRegister = NewAppSuccess("You don't have an account.Please sign up", "SS-11000", nil)
	SuccessLogin    = NewAppSuccess("you  have an account !!!", "SS-12000", nil)
)

type AppSuccess struct {
	Status  bool        `json:"status"`
	Message string      `json:"message,omitempty"`
	Code    string      `json:"code,omitempty"`
	Data    interface{} `json:"data"`
}

func (s *AppSuccess) Error() string {
	//TODO implement me
	panic("implement me")
}

func (s *AppSuccess) Success() string {
	return s.Message
}

func (s *AppSuccess) Marshal() []byte {
	marshal, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return marshal
}

func NewAppSuccess(message, code string, data interface{}) *AppSuccess {
	return &AppSuccess{
		Status:  true,
		Message: message,
		Code:    code,
		Data:    data,
	}
}
