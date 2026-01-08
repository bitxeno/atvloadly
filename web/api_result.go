package web

type ApiResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
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
