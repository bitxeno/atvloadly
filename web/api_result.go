package web

import (
	"errors"

	"github.com/bitxeno/atvloadly/internal/signing"
)

type ApiResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// SigningErrorData is the data payload of a failed external signing request.
// Code is a stable signing code the frontend translates; Msg of the wrapping
// ApiResult keeps the English diagnostic.
type SigningErrorData struct {
	Code     string          `json:"code"`
	Class    signing.Class   `json:"class"`
	Issues   []signing.Issue `json:"issues,omitempty"`
	AppCount int64           `json:"app_count,omitempty"`
}

func apiSuccess(data any) ApiResult {
	return ApiResult{
		Code: 200,
		Msg:  "success",
		Data: data,
	}
}

func apiError(msg string) ApiResult {
	return ApiResult{
		Code: -1,
		Msg:  msg,
	}
}

// apiSigningError returns the error payload for err. A *signing.Error in err's
// chain fills SigningErrorData; any other error falls back to apiError.
func apiSigningError(err error) ApiResult {
	var e *signing.Error
	if !errors.As(err, &e) {
		return apiError(err.Error())
	}
	return ApiResult{
		Code: -1,
		Msg:  err.Error(),
		Data: SigningErrorData{Code: e.Code, Class: e.Class, Issues: e.Issues},
	}
}
