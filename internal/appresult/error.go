package appresult

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

var errStatusMap = map[string]int{
	"SE-00001": http.StatusBadRequest,
	"SE-00002": http.StatusUnauthorized,
	"SE-00003": http.StatusForbidden,
	"SE-00004": http.StatusNotFound,
	"SE-00005": http.StatusConflict,
	"SE-00006": http.StatusInternalServerError,
}

var (
	ErrMissingParam       = NewAppError(nil, "missing param", "SE-00001")
	ErrNotFoundBase       = NewAppError(nil, "not found", "SE-00004")
	ErrInternalServer     = NewAppError(nil, "internal server error", "SE-00006")
	ErrNotAcceptable      = NewAppError(nil, "not acceptable", "SE-00001")
	ErrAlreadyAccount     = NewAppError(nil, "this account has already been registered", "SE-00005")
	ErrInvalidCredentials = NewAppError(nil, "authentication failed", "SE-00002")
	ErrMissingToken       = NewAppError(nil, "no token", "SE-00002")
	ErrBadToken           = NewAppError(nil, "bad token", "SE-00002")
	ErrTokenInBlacklist   = NewAppError(nil, "token is blacklisted", "SE-00002")
	ErrAlreadyDataa       = NewAppError(nil, "not found", "SE-00004")
	ErrForbidden          = NewAppError(nil, "you don't have access", "SE-00003")
	ErrInvalitJson = func(data interface{}) *AppError {
		return NewAppError(nil, fmt.Sprintf("invalid JSON data: %v", data), "SE-00001")
	}
	ErrNotMainImage        = NewAppError(nil, "mainImage required", "SE-00001")
	ErrCreateDir           = NewAppError(nil, "cannot create upload dir", "SE-00006")
	ErrSaveImage           = NewAppError(nil, "cannot save image", "SE-00006")
	ErrRole                = NewAppError(nil, "wrong role", "SE-00003")
	ErrPasswordAgainByName = NewAppError(nil, "already this password", "SE-00005")
	ErrNotImage            = NewAppError(nil, "image required", "SE-00001")
	ErrOverLimitt          = NewAppError(nil, "went over the limit", "SE-00001")
	ErrOTP                 = NewAppError(nil, "your OTP password is incorrect", "SE-00002")
	ErrTimee               = NewAppError(nil, "your time is incorrect", "SE-00001")
	ErrEmpty               = NewAppError(nil, "No available tables of this type", "SE-00004")
	ErrCanSit              = NewAppError(nil, "won't fit in the table", "SE-00001")
	ErrAlreadyCouponn      = NewAppError(nil, "the coupon already used", "SE-00005")
	ErrExpiredCouponn      = NewAppError(nil, "expired coupon", "SE-00001")
	ErrReason              = NewAppError(nil, "need reason", "SE-00001")
	ErrStatus              = NewAppError(nil, "status does not allow changes", "SE-00001")
	ErrWrongs              = NewAppError(nil, "wrong", "SE-00001")
	ErrComment             = NewAppError(nil, "the length of the comment should not exceed 720 characters.", "SE-00001")
	ErrNameOrPhoneNumber   = NewAppError(nil, "name and phone number must not be empty", "SE-00001")
)

var ErrNotFoundType = func(id int, field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("not found, %s id: %v", field , id),
		"SE-00004",
	)
}

var ErrAlreadyData = func(field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("the %s already exists", field ),
		"SE-00004",
	)
}

var ErrOverLimit = func(limit int, field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("%s went over the limit, limit: %d", field , limit),
		"SE-00001",
	)
}

var ErrNotFoundTypeStr = func(field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("not found, %s ", field ),
		"SE-00004",
	)
}

var ErrTime = func(field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("incorrect %s time format", field ),
		"SE-00001",
	)
}

var ErrAlreadyCoupon = func(id int) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("the client coupon id = %d already used", id),
		"SE-00005",
	)
}

var ErrExpiredCoupon = func(id int) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("expired coupon id = %d", id),
		"SE-00001",
	)
}

var ErrWrong = func(field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("wrong %s", field ),
		"SE-00001",
	)
}

var ErrNeed = func(field  string) *AppError {
	return NewAppError(
		nil,
		fmt.Sprintf("%s is required", field ),
		"SE-00001",
	)
}

type AppError struct {
	Status  bool   `json:"status"`
	Err     error  `json:"-"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func (e *AppError) Marshal() []byte {
	marshal, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	return marshal
}

func NewAppError(err error, message, code string) *AppError {
	return &AppError{
		Status:  false,
		Err:     err,
		Message: message,
		Code:    code,
	}
}

func HandleError(c *gin.Context, err error) {
	var appErr *AppError

	if errors.As(err, &appErr) {
		status, ok := errStatusMap[appErr.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		c.JSON(status, appErr)
		return
	}

	c.JSON(http.StatusInternalServerError, ErrInternalServer)
}
