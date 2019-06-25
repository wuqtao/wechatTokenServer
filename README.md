# wechatman
基于fasthttp的微信开发者方便使用的accesstoken管理工具，无需配置redis或者memcached等工具，程序内部自持并保证定时更新accesstoken   
1.接口/query?appid=&token=,提供接口查询最新有效的accesstoken   
2.接口/update?appid=&token,强制更新某appid的accesstoken    
3.接口/reload?token=,提供热加载配置文件，用于添加或者删除appid配置，以及其他配置更改   
   
接口1，2共用ip白名单，接口3为高级权限接口，单独使用ip白名单   
需要注意的是，如果使用nginx配置域名转发，则ip白名单会失效（请求ip地址变成nginx机器的地址）
