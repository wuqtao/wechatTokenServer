package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"strconv"
	"time"
	"wechatman/config"
	"wechatman/wechat"
)
var configFile string
var test bool

func init(){
	flag.StringVar(&configFile,"conf","./wechatman.toml","assign the config file path")
	flag.BoolVar(&test,"test",false,"is test config file")
}

func main(){
	flag.Parse()
	conf,err := config.LoadConfig(configFile)
	if err != nil{
		fmt.Println("config file error:"+err.Error())
		return
	}
	config.GetConfigMan().SetConfig(conf)
	conf = config.GetConfigMan().GetConfig()
	//测试配置文件成功
	if test{
		fmt.Println("config file is right")
		return
	}

	wechatman,err := wechat.BuildWechatMan(conf.GetAheadTime(),conf.GetLoopTime(),conf.GetWechatConfigs()...)
	if err != nil{
		fmt.Println(err.Error())
		return
	}
	wechatman.Run()

	if err != nil{
		fmt.Println(err.Error())
		return
	}
	err = fasthttp.ListenAndServe(":"+strconv.Itoa(conf.GetPort()),requesthandler)
	if err != nil{
		fmt.Println(err.Error())
		return
	}
	fmt.Println("server start success bind port "+strconv.Itoa(conf.GetPort()))
}

type Result struct{
	AccessToken string `json:"accessToken"`
	Msg string         `json:"msg"`
	ServerTime int64	   `json:"serverTime"`
	ExpireAt   int64     `json:"expireAt"`
}

func requesthandler(ctx *fasthttp.RequestCtx){
	if !ctx.IsGet(){
		ctx.Response.SetBody([]byte("only get supported"))
		return
	}
	switch(string(ctx.Path())){
		case "/query":
			if !ctx.QueryArgs().Has("appid") || !ctx.QueryArgs().Has("token"){
				ctx.Response.SetBody([]byte("param not enough"))
				return
			}

			if !QueryIpAuth(ctx.RemoteIP().String()){
				ctx.Response.SetBody([]byte("ip not in white list"))
				return
			}

			appid := ctx.QueryArgs().Peek("appid")
			token := ctx.QueryArgs().Peek("token")
			result := Result{
				ServerTime:time.Now().Unix(),
			}
			wechatman,err := wechat.GetWechatMan()
			if err != nil{
				fmt.Println(err.Error())
				return
			}
			accessToken,expireAt,err := wechatman.QueryAccessToken(string(appid),string(token))
			if err != nil{
				result.Msg = err.Error()
			}else{
				result.Msg = "success"
			}
			result.ExpireAt = expireAt
			result.AccessToken = accessToken
			res,err := json.Marshal(result)
			if err !=nil{
				ctx.Response.SetBody([]byte(err.Error()))
			}else{
				ctx.Response.SetBody(res)
			}

			break
		case "/update":
			if !ctx.QueryArgs().Has("appid") || !ctx.QueryArgs().Has("token"){
				ctx.Response.SetBody([]byte("param not enough"))
				return
			}

			if !QueryIpAuth(ctx.RemoteIP().String()){
				ctx.Response.SetBody([]byte("ip not in white list"))
				return
			}

			appid := ctx.QueryArgs().Peek("appid")
			token := ctx.QueryArgs().Peek("token")

			wechatman,err := wechat.GetWechatMan()
			if err != nil{
				fmt.Println(err.Error())
				return
			}
			_,_,err = wechatman.QueryAccessToken(string(appid),string(token))
			if err != nil{
				ctx.Response.SetBody([]byte("{\"msg\":\""+err.Error()+"\"}"))
				return
			}
			wechatman.ForceRefreshAccessToken(string(appid))
			ctx.Response.SetBody([]byte("{\"msg\":\"success\"}"))

			break
	case "/reload":
		if !ctx.QueryArgs().Has("token"){
			ctx.Response.SetBody([]byte("param not enough"))
			return
		}

		if !ReloadIpAuth(ctx.RemoteIP().String()){
			ctx.Response.SetBody([]byte("ip not in white list"))
			return
		}
		token := string(ctx.QueryArgs().Peek("token"))
		adminToken := config.GetConfigMan().GetConfig().GetAdminToken()
		if string(token) != adminToken{
			ctx.Response.SetBody([]byte("token error"))
			return
		}

		conf,err := config.LoadConfig(configFile)
		if err != nil{
			ctx.Response.SetBody([]byte(err.Error()))
			return
		}

		config.GetConfigMan().SetConfig(conf)
		wechatMan,err := wechat.GetWechatMan()
		if err != nil {
			ctx.Response.SetBody([]byte(err.Error()))
			return
		}

		go func (){
			fmt.Println("reload request: "+time.Now().String())
			err = wechatMan.Rebuild(conf.GetAheadTime(),conf.GetLoopTime(),conf.GetWechatConfigs()...)
			if err != nil{
				fmt.Println("rebuild error "+err.Error())
				return
			}
		}()
		ctx.Response.SetBody([]byte("config is reloading"))
		break
	default:
			ctx.Response.SetBody([]byte("no this route"))
	}
}

func QueryIpAuth(ip string) bool{
	conf := config.GetConfigMan().GetConfig()
	conf.RLock()
	if !conf.UseIpWhiteList{
		return true
	}
	iplist := conf.GetIpList()
	for _,ipSet := range iplist{
		if ipSet == ip {
			conf.RUnlock()
			return true
		}
	}
	conf.RUnlock()
	return false
}

func ReloadIpAuth(ip string) bool{
	conf := config.GetConfigMan().GetConfig()
	conf.RLock()
	iplist := conf.GetAdminIpList()
	for _,ipSet := range iplist{
		if ipSet == ip {
			conf.RUnlock()
			return true
		}
	}
	conf.RUnlock()
	return false
}

