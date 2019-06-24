package wechat

import "testing"

func TestGetErrorCode(test *testing.T){
	if GetErrorMsg(-1) != "系统繁忙，此时请开发者稍候再试"{
		test.Error("code -1 error")
	}

	if GetErrorMsg(0) != "请求成功"{
		test.Error("code 0 error")
	}

	if GetErrorMsg(40005) != "不合法的文件类型" {
		test.Error("code -1 error")
	}
}
