package core

type WechatConfig struct {
	AppID string          //微信appid
	AppSecret string      //微信appsecret
	OriginID string 	  //原始id
	Token string          //微信后台token
	EncodingAESKey string //消息加解密秘钥
	Encryption bool       //消息是否加密
}

