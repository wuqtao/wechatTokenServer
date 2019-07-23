package wechat

import (
	"errors"
	"fmt"
	"github.com/dbldqt/util"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
	"log"
	"strconv"
	"sync"
	"time"
)

const (
	ACCESS_TOKEN_API  = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
)
//微信配置信息
type WechatConfig struct {
	AppID string          //微信appid
	AppSecret string      //微信appsecret
	Token string          //查询校验token
	NotifyUrl []string	  //accessToken更新后的通知url
}
//定义微信应用，每个微信配置看做不同的应用
type WechatApp struct {
	locker       sync.RWMutex
	WechatConfig *WechatConfig
	accessToken  string
	updateTime   time.Time
	duration     time.Duration
	aheadTime int
	deleted bool
	needUpdate bool
}

func (wa *WechatApp)GetAccessToken() string{
	return wa.accessToken
}

func (wa *WechatApp)GetWechatConfig() *WechatConfig{
	return wa.WechatConfig
}

func (wa *WechatApp)GetUpDateTime() time.Time{
	return wa.updateTime
}

func (wa *WechatApp)GetDuration() time.Duration{
	return wa.duration
}

func (wa *WechatApp) UpdateAccessToken(wg *sync.WaitGroup){
	_,resp,error := fasthttp.Get(nil,fmt.Sprintf(ACCESS_TOKEN_API,wa.WechatConfig.AppID,wa.WechatConfig.AppSecret))
	if error != nil{
		return
	}
	nowTime := time.Now()
	jre := gjson.Parse(string(resp))
	if jre.Get("access_token").Exists(){
		wa.locker.Lock()
		wa.needUpdate = false
		wa.accessToken = jre.Get("access_token").String()
		log.Println(wa.WechatConfig.AppID+":"+wa.accessToken)
		num,err := strconv.Atoi(jre.Get("expires_in").String())
		if err == nil{
			num = num-wa.aheadTime  //提前一定时间去更新
			wa.duration = time.Second*time.Duration(num)
			wa.updateTime = nowTime
		}else{
			log.Println("prase accesstoken expire error "+err.Error())
			num = 0
			wa.duration = time.Nanosecond
			wa.updateTime = nowTime
		}
		for _,url := range wa.WechatConfig.NotifyUrl{
			if url != ""{
				go func(){
					resp,err := util.PostFields(url,map[string]string{
						"accessToken":wa.accessToken,
						"updateTime":strconv.FormatInt(wa.updateTime.Unix(),10),
						"expires_in":strconv.Itoa(num),
					})
					if err != nil{
						log.Println(url+" notify accessToken update err url:"+err.Error())
					}
					log.Println(url+" notify url response "+string(resp))
				}()
			}
		}
		wa.locker.Unlock()
	}else{
		log.Println("request accesstoken error"+string(resp))
		errcode := int(jre.Get("errcode").Int())
		errmsg := GetErrorMsg(errcode)
		if errmsg == ERROR_UNKONWN{
			errmsg = jre.Get("errmsg").String()
		}
		log.Println(strconv.Itoa(errcode)+":"+errmsg)
	}
	wg.Done()
}

func NewWechatApp(wc *WechatConfig,aheadTime int) *WechatApp{
	wa := &WechatApp{
		WechatConfig: wc,
		aheadTime:aheadTime,
	}
	return wa
}

type WechatMan struct{
	sync.RWMutex
	apps []*WechatApp
	isRuning bool             //标志wechatApp是否已经运行
	loopStopChan chan int     //控制loopAccessToken结束
	aheadTime int
	loopTime int
}

func (wm *WechatMan) AddWehcatApp(wa ...*WechatApp){
	wm.Lock()
	wm.apps = append(wm.apps,wa...)
	wm.Unlock()
}

func (wm *WechatMan) DelWechatAppByAppID(appID string){
	wm.Lock()
	newAPPs := make([]*WechatApp,0)
	for i,wa:= range wm.apps{
		if wa.WechatConfig.AppID != appID{
			newAPPs = append(newAPPs,wm.apps[i])
		}
	}
	wm.apps = newAPPs
	wm.Unlock()
}

func (wm *WechatMan) loopAccessToken(stopCh <-chan int){
	loopChan := make(chan int,1)
	stopLoop := make(chan int,1)
	go func(){
		log.Println("send signal routine start")
	loopsig:
		for{
			select{
				case <-stopLoop:
					break loopsig
				default:
					loopChan<-1
					time.Sleep(time.Second*time.Duration(wm.loopTime))
			}
		}
		log.Println("send signal routine end")
	}()
	go func(){
		log.Println("loop routine start")
loop:
		for{
			//此处使用两个chan保证第一时间接收信号，如果使用default:time.sleep()会阻断ch消息接收，
			//使用两个ch可以保证消息第一时间接收
			select{
				case <-stopCh://此处接收到消息后，wm.stop方法即返回不会阻塞后续流程，此处的退出流程无需考虑时效性
					log.Println("loop routine end")
					//收到stopCh信号后，此处将退出，如果不通知发送轮训信号的携程，那个携程将一直阻塞在发送消息状态
					//虽然发送方使用了time.sleep，但是这里不要求时效性，只要保证携程能够结束不一直阻塞即可
					stopLoop<-1
					break loop
				case <-loopChan:
					wm.refreshAccessToken()
			}
		}
	}()
}

func (wm *WechatMan) Run() error{
	if wm.isRuning {
		return errors.New("wechatMan was runing")
	}

	wm.Lock()
	wm.isRuning = true
	wm.Unlock()
	wm.loopAccessToken(wm.loopStopChan)
	return nil
}

func (wm *WechatMan) Stop(){
	wm.loopStopChan<-1
	wm.Lock()
	wm.isRuning = false
	wm.Unlock()
}

func (wm *WechatMan) refreshAccessToken(){
	wg := sync.WaitGroup{}
	wm.RLock()
	for _,app := range wm.apps{
		if app != nil{
			app.locker.RLock()
			if app.needUpdate || time.Since(app.updateTime) >= app.duration &&
							 app.WechatConfig.Token != "" &&
							 app.WechatConfig.AppSecret != ""{
				//此处为了多个微信公众号时提高更新效率，启用子进程更新，
				//由于外层有加锁和解锁操作，所以需要使用wg同步进程状态
				wg.Add(1)
				app.locker.RUnlock()
				go app.UpdateAccessToken(&wg)
			}else{
				app.locker.RUnlock()
			}
		}
	}
	wm.RUnlock()
	wg.Wait()

}

func (wm *WechatMan) ForceRefreshAccessToken(appids ...string){
	wg := sync.WaitGroup{}
	wm.RLock()
	for _,appid := range appids{
		for _,app := range wm.apps{
			if app != nil{
				app.locker.RLock()
				if app.WechatConfig.AppID == appid{
					//此处为了多个微信公众号时提高更新效率，启用子进程更新，
					//由于外层有加锁和解锁操作，所以需要使用wg同步进程状态
					wg.Add(1)
					app.locker.RUnlock()
					go app.UpdateAccessToken(&wg)
				}else{
					app.locker.RUnlock()
				}
			}
		}
	}
	wm.RUnlock()
	wg.Wait()

}

func (wm *WechatMan) Rebuild(aheadTime,loopTime int,wxconfs ...*WechatConfig) error{
	wm.Stop()
	wm.Lock()
	wm.aheadTime = aheadTime
	wm.loopTime = loopTime
	//标记删除的app
	for _,app := range wm.apps{
		app.locker.Lock()
		app.deleted = true
		for _,wxconf := range wxconfs{
			if wxconf.AppID == app.WechatConfig.AppID{
				app.deleted = false
				break
			}
		}
		app.locker.Unlock()
	}
	for _,wxconf := range wxconfs{
		isNew := true
		for _,app := range wm.apps{
			app.locker.Lock()
			//app已标记为删除
			if app.deleted {
				app.locker.Unlock()
				continue
			}
			//已经存在的app
			if wxconf.AppID == app.WechatConfig.AppID{
				isNew = false
				app.aheadTime = aheadTime
				if (app.WechatConfig.AppSecret != wxconf.AppSecret){
					app.WechatConfig.AppSecret = wxconf.AppSecret
					app.needUpdate = true
				}
				app.WechatConfig.Token = wxconf.Token
				app.locker.Unlock()
				break
			}
			app.locker.Unlock()
		}

		if isNew{
			wm.apps = append(wm.apps,NewWechatApp(wxconf,aheadTime))
		}
	}
	wm.Unlock()
	wm.Run()
	log.Println("reload success "+time.Now().String())
	return nil
}

func (wm *WechatMan) QueryAccessToken(appid,token string) (string,int64,error){
	var accesstoken string
	var expireAt int64
	var err error
	wm.RLock()
	for _,app := range wm.apps{
		app.locker.RLock()
		if app.WechatConfig.AppID == appid && app.WechatConfig.Token == token{
			if app.deleted{
				accesstoken = app.accessToken
				err = errors.New("this app is deleted,can't ensure the accesstoken is valid")
				expireAt = app.updateTime.Add(app.duration).Unix()
			}else{
				accesstoken = app.accessToken
				err = nil
				expireAt = app.updateTime.Add(app.duration).Unix()
			}
		}
		app.locker.RUnlock()
	}
	wm.RUnlock()
	if accesstoken == ""{
		err = errors.New("no accesstoken for this appid and token")
		expireAt = 0
	}
	return accesstoken,expireAt,err

}
var wechatMan *WechatMan

//实现单利模式，返回唯一的WechatMan实例
func BuildWechatMan(aheadTime,loopTime int,wxconfs ...*WechatConfig) (*WechatMan,error){
	if wechatMan != nil{
		return wechatMan,nil
	}
	if len(wxconfs) < 1{
		return nil,errors.New("need one WechatConfig at least")
	}
	wechatMan = &WechatMan{
		apps: []*WechatApp{},
		loopStopChan:make(chan int),
		aheadTime:aheadTime,
		loopTime:loopTime,
	}

	//根据给定的配置初始化wechatapp
	for _,conf := range wxconfs{
		wechatMan.apps = append(wechatMan.apps,NewWechatApp(conf,aheadTime))
	}
	return wechatMan,nil
}

func GetWechatMan() (*WechatMan,error){
	if wechatMan == nil{
		return nil,errors.New("wechatMan not exist,call BuildWechatMan to get it")
	}
	return wechatMan,nil
}