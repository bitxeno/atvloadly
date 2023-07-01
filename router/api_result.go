package router

type ApiResult struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func apiSuccess(data interface{}) ApiResult {
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
