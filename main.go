package main

import (
	"encoding/base64"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/model"
	"parseAndCombineMyClashRules/utils"
	"path/filepath"
	"sync"
)

var absPath string
var absPathErr error

func init() {
	// 设置日志输出前缀
	log.SetPrefix("TRACE: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Llongfile)
	// 可执行程序所在目录
	root := filepath.Dir(os.Args[0])
	absPath, absPathErr = filepath.Abs(root)
	// 创建日志目录
	os.MkdirAll(absPath+"/log", os.ModePerm)
	// 设置日志输出文件
	logFile, logFileErr := os.OpenFile(absPath+"/log/errors.txt",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if logFileErr != nil {
		log.Printf("error: %v\n", logFileErr)
		panic("创建日志文件失败")
	}
	log.SetOutput(logFile)
}

func main() {
	if absPathErr != nil {
		fmt.Println("获取运行路径失败")
		return
	}

	// 获取参数
	inputArgs := os.Args
	if len(inputArgs) < 2 {
		fmt.Println("请输入参数")
		return
	}

	port := inputArgs[1]
	fmt.Println("服务启动，端口：" + port)
	http.HandleFunc("/parse", parseRule)
	http.ListenAndServe("0.0.0.0:"+port, nil)
}

func parseRule(w http.ResponseWriter, r *http.Request) {
	userConfigMap, baseRuleMap, done := parseRequestParams(r)
	if done {
		return
	}

	proxyArr, proxyGroupArr, proxyNameArr, done2 := getProxies(userConfigMap, r)
	if done2 {
		return
	}

	outPutClash(w, userConfigMap, baseRuleMap, proxyArr, proxyGroupArr, proxyNameArr)
}

func outPutClash(w http.ResponseWriter, userConfigMap map[string]interface{}, baseRuleMap map[string]interface{}, proxyArr []map[interface{}]interface{}, proxyGroupArr []map[interface{}]interface{}, proxyNameArr []interface{}) {
	// 构造一些前置的参数
	outputConfig := model.OutputConfig{}

	outputConfig.Port = getConfigFieldValue("port", userConfigMap, baseRuleMap)
	outputConfig.SocksPort = getConfigFieldValue("socks-port", userConfigMap, baseRuleMap)
	outputConfig.RedirPort = getConfigFieldValue("redir-port", userConfigMap, baseRuleMap)
	outputConfig.MixedPort = getConfigFieldValue("mixed-port", userConfigMap, baseRuleMap)

	authentication := getConfigFieldValue("authentication", userConfigMap, baseRuleMap)
	if authentication != nil {
		outputConfig.Authentication = authentication.([]interface{})
	} else {
		outputConfig.Authentication = nil
	}

	outputConfig.AllowLan = getConfigFieldValue("allow-lan", userConfigMap, baseRuleMap)
	outputConfig.BindAddress = getConfigFieldValue("bind-address", userConfigMap, baseRuleMap)
	outputConfig.Mode = getConfigFieldValue("mode", userConfigMap, baseRuleMap)
	outputConfig.LogLevel = getConfigFieldValue("log-level", userConfigMap, baseRuleMap)
	outputConfig.Ipv6 = getConfigFieldValue("ipv6", userConfigMap, baseRuleMap)
	outputConfig.ExternalController = getConfigFieldValue("external-controller", userConfigMap, baseRuleMap)
	outputConfig.ExternalUi = getConfigFieldValue("external-ui", userConfigMap, baseRuleMap)
	outputConfig.Secret = getConfigFieldValue("secret", userConfigMap, baseRuleMap)
	outputConfig.InterfaceName = getConfigFieldValue("interface-name", userConfigMap, baseRuleMap)

	hosts := getConfigFieldValue("hosts", userConfigMap, baseRuleMap)
	if hosts != nil {
		outputConfig.Hosts = hosts.(map[interface{}]interface{})
	} else {
		outputConfig.Hosts = nil
	}

	dns := getConfigFieldValue("dns", userConfigMap, baseRuleMap)
	if dns != nil {
		outputConfig.Dns = dns.(map[interface{}]interface{})
	} else {
		outputConfig.Dns = nil
	}

	outputConfig.Rules = getConfigFieldMergeValueArr("rules", userConfigMap, baseRuleMap)

	outputConfig.RuleProviders = getConfigFieldMergeValueMap("rule-providers", userConfigMap, baseRuleMap)
	// proxy providers
	filterProxyProvier := userConfigMap["filter-proxy-providers"]
	var filterProxyProviderArr []interface{}
	if filterProxyProvier != nil {
		filterProxyProviderArr = filterProxyProvier.([]interface{})
	}
	outputConfig.ProxyProviders = getConfigFieldMergeValueMap("proxy-providers", userConfigMap, baseRuleMap)
	for providerName := range outputConfig.ProxyProviders {
		for _, proxyProvider := range filterProxyProviderArr {
			if providerName == proxyProvider {
				delete(outputConfig.ProxyProviders, providerName)
			}
		}
	}
	// 构造 proxy
	outputConfig.Proxies = proxyArr
	// 构造 proxyGroup
	filterProxyGroup := userConfigMap["filter-proxy-groups"]
	var filterProxyGroupArr []interface{}
	if filterProxyGroup != nil {
		filterProxyGroupArr = filterProxyGroup.([]interface{})
	}
	var outputProxyGroupMap []map[interface{}]interface{}
	// 加入上面几组订阅的单独group
	outputProxyGroupMap = append(outputProxyGroupMap, proxyGroupArr...)
	userConfigProxyGroups := userConfigMap["proxy-groups"]
	if userConfigProxyGroups != nil {
		userConfigProxyGroupMapArr := generateProxyNameToGroup(userConfigProxyGroups, proxyNameArr, filterProxyGroupArr)
		outputProxyGroupMap = append(outputProxyGroupMap, userConfigProxyGroupMapArr...)
	}
	baseRuleProxyGroups := baseRuleMap["proxy-groups"]
	if baseRuleProxyGroups != nil {
		baseRuleProxyGroupMapArr := generateProxyNameToGroup(baseRuleProxyGroups.([]interface{}), proxyNameArr, filterProxyGroupArr)
		// 重新整理一下需要把一些 group name 加入到 select 组中
		var noneSelectGroupName []interface{}
		for _, proxyGroupMap := range baseRuleProxyGroupMapArr {
			if proxyGroupMap["type"] != "select" {
				noneSelectGroupName = append(noneSelectGroupName, proxyGroupMap["name"])
			}
		}
		for idx, proxyGroupMap := range baseRuleProxyGroupMapArr {
			if proxyGroupMap["type"] == "select" {
				baseRuleProxyGroupMapArr[idx]["proxies"] = append(noneSelectGroupName, baseRuleProxyGroupMapArr[idx]["proxies"].([]interface{})...)
			}
		}
		outputProxyGroupMap = append(outputProxyGroupMap, baseRuleProxyGroupMapArr...)
	}

	outputConfig.ProxyGroups = outputProxyGroupMap

	marshalRule, _ := yaml.Marshal(&outputConfig)
	fmt.Fprintln(w, string(marshalRule))
}

func getProxies(userConfigMap map[string]interface{}, r *http.Request) ([]map[interface{}]interface{}, []map[interface{}]interface{}, []interface{}, bool) {
	// 获取配置文件输入的订阅地址
	pullProxySource := userConfigMap["pull-proxy-source"]
	if pullProxySource == nil {
		return nil, nil, nil, true
	}
	pullProxySourceMap := pullProxySource.([]interface{})
	pullProxySourceMapLength := len(pullProxySourceMap)
	if pullProxySourceMapLength <= 0 {
		log.Printf("请求url: %s，请求IP: %s\n，没有获取到订阅地址", r.URL, utils.GetRequestIp(r))
	}
	// 多线程获取订阅
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(pullProxySourceMapLength)
	var proxyArr []map[interface{}]interface{}
	var proxyGroupArr []map[interface{}]interface{}
	var proxyNameArr []interface{}
	for _, proxySource := range pullProxySourceMap {
		go func(proxySource interface{}) {
			defer waitGroup.Done()
			proxySourceMap := proxySource.(map[interface{}]interface{})
			// 获取订阅
			httpResponseBytes, httpRequestErr := utils.HttpGet(proxySourceMap["url"].(string))
			if httpRequestErr != nil {
				log.Printf(
					"请求url: %s，请求IP: %s, 订阅地址: %s, 报错内容：%v\n，通过订阅地址获取内容失败",
					r.URL,
					utils.GetRequestIp(r),
					proxySourceMap["url"].(string),
					httpRequestErr.Error(),
				)
			}
			filterProxyName := userConfigMap["filter-proxy-name"]
			var filterProxyNameArr []interface{}
			if filterProxyName != nil {
				filterProxyNameArr = filterProxyName.([]interface{})
			}

			filterProxyServer := userConfigMap["filter-proxy-server"]
			var filterProxyServerArr []interface{}
			if filterProxyServer != nil {
				filterProxyServerArr = filterProxyServer.([]interface{})
			}

			decodeBase64Proxy, decodeBase64ProxyErr := base64.RawURLEncoding.DecodeString(string(httpResponseBytes))
			if decodeBase64ProxyErr != nil {
				// 在使用另一个base64 解析
				decodeStdBase64Proxy, decodeStdBase64ProxyErr := base64.RawStdEncoding.DecodeString(string(httpResponseBytes))
				if decodeStdBase64ProxyErr != nil {
					yamlProxyServerArr, yamlProxyServerErr := utils.ParseYamlProxy(
						httpResponseBytes,
						filterProxyNameArr,
						filterProxyServerArr,
					)
					if yamlProxyServerErr != nil {
						log.Printf(
							"请求url: %s，请求IP: %s, 订阅地址: %s\n，解析base64 和 yaml 全部失败",
							r.URL,
							utils.GetRequestIp(r),
							proxySourceMap["url"].(string),
						)
					}
					proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(yamlProxyServerArr, proxySourceMap["name"])
					proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
					proxyArr = append(proxyArr, yamlProxyServerArr...)
					proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
					return
				}
				// 不是 base64 ,解析yaml
				base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(
					decodeStdBase64Proxy,
					filterProxyNameArr,
					filterProxyServerArr,
				)
				if base64ProxyServerErr != nil {
					return
				}

				proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(base64ProxyServerArr, proxySourceMap["name"])
				proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
				proxyArr = append(proxyArr, base64ProxyServerArr...)
				proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
				return
			}
			//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
			base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(
				decodeBase64Proxy,
				filterProxyNameArr,
				filterProxyServerArr,
			)
			if base64ProxyServerErr != nil {
				return
			}

			proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(base64ProxyServerArr, proxySourceMap["name"])
			proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
			proxyArr = append(proxyArr, base64ProxyServerArr...)
			proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
		}(proxySource)
	}
	waitGroup.Wait()
	return proxyArr, proxyGroupArr, proxyNameArr, false
}

func parseRequestParams(r *http.Request) (map[string]interface{}, map[string]interface{}, bool) {
	// 获取配置参数
	configFileNameArr, getConfigFileNameOk := r.URL.Query()["name"]
	if getConfigFileNameOk == false {
		return nil, nil, true
	}
	configFileName := configFileNameArr[0]
	configFile, readConfigFileErr := ioutil.ReadFile(absPath + "/config/" + configFileName + ".yaml")
	if readConfigFileErr != nil {
		log.Printf("error: %v\n", readConfigFileErr)
		log.Println("读入配置文件失败了，检查配置文件是否存在")
		return nil, nil, true
	}
	// 解析本地配置文件
	userConfigMap := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(configFile), &userConfigMap)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// 获取请求参数的配置项
	baseConfigName := r.URL.Query().Get("baseName")
	baseConfigRequestUrl := ""

	if baseConfigName != "" {
		// 如果获取成功就去找配置文件
		for _, baseConfig := range userConfigMap["base-config"].([]interface{}) {
			baseConfigMap := baseConfig.(map[interface{}]interface{})
			if baseConfigMap["name"] == baseConfigName {
				baseConfigRequestUrl = baseConfigMap["url"].(string)
			}
		}
	} else {
		log.Printf("请求url: %s，请求IP: %s\n，没有输入配置文件名称", r.URL, utils.GetRequestIp(r))
		return nil, nil, true
	}

	if baseConfigRequestUrl == "" {
		log.Printf("请求url: %s，请求IP: %s\n，没有找到对应配置文件", r.URL, utils.GetRequestIp(r))
	}

	// 拉取基础配置
	baseRuleBody, baseRuleErr := utils.HttpGet(baseConfigRequestUrl)
	if baseRuleErr != nil {
		log.Println(baseRuleErr)
		log.Printf("请求url: %s，请求IP: %s\n，拉取基础规则失败，请检查网络！！！！", r.URL, utils.GetRequestIp(r))
		return nil, nil, true
	}
	//baseRuleBody := []byte("#---------------------------------------------------#\n## 配置文件需要放置在 $HOME/.config/clash/config.yml\n##\n## 如果您不知道如何操作，请参阅 SS-Rule-Snippet 的 Wiki：\n## https://github.com/Hackl0us/SS-Rule-Snippet/wiki/clash(X)\n#---------------------------------------------------#\n\n# HTTP 代理端口\nport: 7890\n\n# SOCKS5 代理端口\nsocks-port: 7891\n\n# Linux 和 macOS 的 redir 透明代理端口 (重定向 TCP 和 TProxy UDP 流量)\n# redir-port: 7892\n\n# Linux 的透明代理端口（适用于 TProxy TCP 和 TProxy UDP 流量)\n# tproxy-port: 7893\n\n# HTTP(S) and SOCKS5 共用端口\n# mixed-port: 7890\n\n# 本地 SOCKS5/HTTP(S) 服务验证\n# authentication:\n#  - \"user1:pass1\"\n#  - \"user2:pass2\"\n\n# 允许局域网的连接（可用来共享代理）\nallow-lan: true\nbind-address: \"*\"\n# 此功能仅在 allow-lan 设置为 true 时生效，支持三种参数：\n# \"*\"                           绑定所有的 IP 地址\n# 192.168.122.11                绑定一个的 IPv4 地址\n# \"[aaaa::a8aa:ff:fe09:57d8]\"   绑定一个 IPv6 地址\n\n# Clash 路由工作模式\n# 规则模式：rule（规则） / global（全局代理）/ direct（全局直连）\nmode: rule\n\n# Clash 默认将日志输出至 STDOUT\n# 设置日志输出级别 (默认级别：silent，即不输出任何内容，以避免因日志内容过大而导致程序内存溢出）。\n# 5 个级别：silent / info / warning / error / debug。级别越高日志输出量越大，越倾向于调试，若需要请自行开启。\nlog-level: silent\n\n# clash 的 RESTful API 监听地址\nexternal-controller: 127.0.0.1:9090\n\n# 存放配置文件的相对路径，或存放网页静态资源的绝对路径\n# Clash core 将会将其部署在 http://{{external-controller}}/ui\n# external-ui: folder\n\n# RESTful API 的口令 (可选)\n# 通过 HTTP 头中 Authorization: Bearer ${secret} 参数来验证口令\n# 当 RESTful API 的监听地址为 0.0.0.0 时，请务必设定口令以保证安全\n# secret: \"\"\n\n# 出站网卡接口\n# interface-name: en0\n\n# DNS 服务器和建立连接时的 静态 Hosts, 仅在 dns.enhanced-mode 模式为 redir-host 生效\n# 支持通配符域名 (例如: *.clash.dev, *.foo.*.example.com )\n# 不使用通配符的域名优先级高于使用通配符的域名 (例如: foo.example.com > *.example.com > .example.com )\n# 注意: +.foo.com 的效果等同于 .foo.com 和 foo.com\nhosts:\n# '*.clash.dev': 127.0.0.1\n# '.dev': 127.0.0.1\n# 'alpha.clash.dev': '::1'\n\n# DNS 服务器配置(可选；若不配置，程序内置的 DNS 服务会被关闭)\ndns:\n  enable: true\n  listen: 0.0.0.0:53\n  ipv6: true # 当此选项为 false 时, AAAA 请求将返回空\n\n  # 以下填写的 DNS 服务器将会被用来解析 DNS 服务的域名\n  # 仅填写 DNS 服务器的 IP 地址\n  default-nameserver:\n    - 223.5.5.5\n    - 114.114.114.114\n  enhanced-mode: fake-ip # 或 redir-host\n  fake-ip-range: 198.18.0.1/16 # Fake IP 地址池 (CIDR 形式)\n  # use-hosts: true # 查询 hosts 并返回 IP 记录\n\n  # 在以下列表的域名将不会被解析为 fake ip，这些域名相关的解析请求将会返回它们真实的 IP 地址\n  fake-ip-filter:\n    # 以下域名列表参考自 vernesong/OpenClash 项目，并由 Hackl0us 整理补充\n    # === LAN ===\n    - '*.lan'\n    # === Linksys Wireless Router ===\n    - '*.linksys.com'\n    - '*.linksyssmartwifi.com'\n    # === Apple Software Update Service ===\n    - 'swscan.apple.com'\n    - 'mesu.apple.com'\n    # === Windows 10 Connnect Detection ===\n    - '*.msftconnecttest.com'\n    - '*.msftncsi.com'\n    # === NTP Service ===\n    - 'time.*.com'\n    - 'time.*.gov'\n    - 'time.*.edu.cn'\n    - 'time.*.apple.com'\n\n    - 'time1.*.com'\n    - 'time2.*.com'\n    - 'time3.*.com'\n    - 'time4.*.com'\n    - 'time5.*.com'\n    - 'time6.*.com'\n    - 'time7.*.com'\n\n    - 'ntp.*.com'\n    - 'ntp.*.com'\n    - 'ntp1.*.com'\n    - 'ntp2.*.com'\n    - 'ntp3.*.com'\n    - 'ntp4.*.com'\n    - 'ntp5.*.com'\n    - 'ntp6.*.com'\n    - 'ntp7.*.com'\n\n    - '*.time.edu.cn'\n    - '*.ntp.org.cn'\n    - '+.pool.ntp.org'\n\n    - 'time1.cloud.tencent.com'\n    # === Music Service ===\n    ## NetEase\n    - '+.music.163.com'\n    - '*.126.net'\n    ## Baidu\n    - 'musicapi.taihe.com'\n    - 'music.taihe.com'\n    ## Kugou\n    - 'songsearch.kugou.com'\n    - 'trackercdn.kugou.com'\n    ## Kuwo\n    - '*.kuwo.cn'\n    ## JOOX\n    - 'api-jooxtt.sanook.com'\n    - 'api.joox.com'\n    - 'joox.com'\n    ## QQ\n    - '+.y.qq.com'\n    - '+.music.tc.qq.com'\n    - 'aqqmusic.tc.qq.com'\n    - '+.stream.qqmusic.qq.com'\n    ## Xiami\n    - '*.xiami.com'\n    ## Migu\n    - '+.music.migu.cn'\n    # === Game Service ===\n    ## Nintendo Switch\n    - '+.srv.nintendo.net'\n    ## Sony PlayStation\n    - '+.stun.playstation.net'\n    ## Microsoft Xbox\n    - 'xbox.*.microsoft.com'\n    - '+.xboxlive.com'\n    # === Other ===\n    ## QQ Quick Login\n    - 'localhost.ptlogin2.qq.com'\n    ## Golang\n    - 'proxy.golang.org'\n    ## STUN Server\n    - 'stun.*.*'\n    - 'stun.*.*.*'\n\n\n    ## Bilibili CDN\n    - '*.mcdn.bilivideo.cn'\n\n  # 支持 UDP / TCP / DoT / DoH 协议的 DNS 服务，可以指明具体的连接端口号。\n  # 所有 DNS 请求将会直接发送到服务器，不经过任何代理。\n  # Clash 会使用最先获得的解析记录回复 DNS 请求\n  nameserver:\n    - https://doh.pub/dns-query\n    - https://dns.alidns.com/dns-query\n\n  # 当 fallback 参数被配置时, DNS 请求将同时发送至上方 nameserver 列表和下方 fallback 列表中配置的所有 DNS 服务器.\n  # 当解析得到的 IP 地址的地理位置不是 CN 时，clash 将会选用 fallback 中 DNS 服务器的解析结果。\n  # fallback:\n  #   - https://dns.google/dns-query\n\n  # 如果使用 nameserver 列表中的服务器解析的 IP 地址在下方列表中的子网中，则它们被认为是无效的，\n  # Clash 会选用 fallback 列表中配置 DNS 服务器解析得到的结果。\n  #\n  # 当 fallback-filter.geoip 为 true 且 IP 地址的地理位置为 CN 时，\n  # Clash 会选用 nameserver 列表中配置 DNS 服务器解析得到的结果。\n  #\n  # 当 fallback-filter.geoip 为 false, 如果解析结果不在 fallback-filter.ipcidr 范围内，\n  # Clash 总会选用 nameserver 列表中配置 DNS 服务器解析得到的结果。\n  #\n  # 采取以上逻辑进行域名解析是为了对抗 DNS 投毒攻击。\n  fallback-filter:\n    geoip: false\n    ipcidr:\n      - 240.0.0.0/4\n      - 0.0.0.0/32\n    # domain:\n    #   - '+.google.com'\n    #   - '+.facebook.com'\n    #   - '+.youtube.com'\n\nproxies:\n  # shadowsocks\n  # 支持加密方式：\n  #   aes-128-gcm aes-192-gcm aes-256-gcm\n  #   aes-128-cfb aes-192-cfb aes-256-cfb\n  #   aes-128-ctr aes-192-ctr aes-256-ctr\n  #   rc4-md5 chacha20 chacha20-ietf xchacha20\n  #   chacha20-ietf-poly1305 xchacha20-ietf-poly1305\n\n  - name: \"ss1\"\n    type: ss\n    server: server\n    port: 443\n    cipher: chacha20-ietf-poly1305\n    password: \"password\"\n    # udp: true\n\n  - name: \"ss2\"\n    type: ss\n    server: server\n    port: 443\n    cipher: AEAD_CHACHA20_POLY1305\n    password: \"password\"\n    plugin: obfs\n    plugin-opts:\n      mode: tls # 混淆模式，可以选择 http 或 tls\n      host: bing.com # 混淆域名，需要和服务器配置保持一致\n\n  - name: \"ss3\"\n    type: ss\n    server: server\n    port: 443\n    cipher: AEAD_CHACHA20_POLY1305\n    password: \"password\"\n    plugin: v2ray-plugin\n    plugin-opts:\n      mode: websocket # 暂时不支持 QUIC 协议\n      # tls: true # wss\n      # skip-cert-verify: true\n      # host: bing.com\n      # path: \"/\"\n      # headers:\n      #   custom: value\n\n  # vmess\n  # 支持加密方式：auto / aes-128-gcm / chacha20-poly1305 / none\n  - name: \"vmess\"\n    type: vmess\n    server: server\n    port: 443\n    uuid: uuid\n    alterId: 32\n    cipher: auto\n    # udp: true\n    # tls: true\n    # skip-cert-verify: true\n    # servername: example.com # 优先级高于 wss host\n    # network: ws\n    # ws-path: /path\n    # ws-headers:\n    #   Host: v2ray.com\n\n  - name: \"vmess-http\"\n    type: vmess\n    server: server\n    port: 443\n    uuid: uuid\n    alterId: 32\n    cipher: auto\n    # udp: true\n    # network: http\n    # http-opts:\n    #   # method: \"GET\"\n    #   # path:\n    #   #   - '/'\n    #   #   - '/video'\n    #   # headers:\n    #   #   Connection:\n    #   #     - keep-alive\n\n  # socks5\n  - name: \"socks\"\n    type: socks5\n    server: server\n    port: 443\n    # username: username\n    # password: password\n    # tls: true\n    # skip-cert-verify: true\n    # udp: true\n\n  # http\n  - name: \"http\"\n    type: http\n    server: server\n    port: 443\n    # username: username\n    # password: password\n    # tls: true # https\n    # skip-cert-verify: true\n\n  # snell (注意：暂时不支持 UDP 转发)\n  - name: \"snell\"\n    type: snell\n    server: server\n    port: 44046\n    psk: yourpsk\n    # obfs-opts:\n    # mode: http # 或 tls\n    # host: bing.com\n\n  # Trojan\n  - name: \"trojan\"\n    type: trojan\n    server: server\n    port: 443\n    password: yourpsk\n    # udp: true\n    # sni: example.com # 服务名称\n    # alpn:\n    #   - h2\n    #   - http/1.1\n    # skip-cert-verify: true\n\n  # ShadowsocksR\n  # 支持的加密方式: SS 中支持的所有流加密方式\n  # 支持的混淆方式:\n  #   plain http_simple http_post\n  #   random_head tls1.2_ticket_auth tls1.2_ticket_fastauth\n  # 支持的协议:\n  #   origin auth_sha1_v4 auth_aes128_md5\n  #   auth_aes128_sha1 auth_chain_a auth_chain_b\n  - name: \"ssr\"\n    type: ssr\n    server: server\n    port: 443\n    cipher: chacha20-ietf\n    password: \"password\"\n    obfs: tls1.2_ticket_auth\n    protocol: auth_sha1_v4\n    # obfs-param: domain.tld\n    # protocol-param: \"#\"\n    # udp: true\n\nproxy-groups:\n  # 代理的转发链, 在 proxies 中不应该包含 relay. 不支持 UDP.\n  # 流量: clash <-> http <-> vmess <-> ss1 <-> ss2 <-> 互联网\n  - name: \"relay\"\n    type: relay\n    proxies:\n      - http\n      - vmess\n      - ss1\n      - ss2\n\n  # url-test 可以自动选择与指定 URL 测速后，延迟最短的服务器\n  - name: \"auto\"\n    type: url-test\n    proxies:\n      - ss1\n      - ss2\n      - vmess1\n    url: 'http://www.gstatic.com/generate_204'\n    interval: 300\n\n  # fallback 可以尽量按照用户书写的服务器顺序，在确保服务器可用的情况下，自动选择服务器\n  - name: \"fallback-auto\"\n    type: fallback\n    proxies:\n      - ss1\n      - ss2\n      - vmess1\n    url: 'http://www.gstatic.com/generate_204'\n    interval: 300\n\n  # load-balance 可以使相同 eTLD 请求在同一条代理线路上\n  - name: \"load-balance\"\n    type: load-balance\n    proxies:\n      - ss1\n      - ss2\n      - vmess1\n    url: 'http://www.gstatic.com/generate_204'\n    interval: 300\n\n  # select 用来允许用户手动选择 代理服务器 或 服务器组\n  # 您也可以使用 RESTful API 去切换服务器，这种方式推荐在 GUI 中使用\n  - name: Proxy\n    type: select\n    proxies:\n      - ss1\n      - ss2\n      - vmess1\n      - auto\n\n  - name: UseProvider\n    type: select\n    use:\n      - provider1\n    proxies:\n      - Proxy\n      - DIRECT\n\nproxy-providers:\n  provider1:\n    type: http\n    url: \"url\"\n    interval: 3600\n    path: ./provider1.yaml\n    health-check:\n      enable: true\n      interval: 600\n      url: http://www.gstatic.com/generate_204\n  test:\n    type: file\n    path: /test.yaml\n    health-check:\n      enable: true\n      interval: 36000\n      url: http://www.gstatic.com/generate_204\n\nrules:\n  # 自定义规则\n  ## 您可以在此处插入您补充的自定义规则（请注意保持缩进）\n\n  # Apple\n  - DOMAIN,safebrowsing.urlsec.qq.com,DIRECT # 如果您并不信任此服务提供商或防止其下载消耗过多带宽资源，可以进入 Safari 设置，关闭 Fraudulent Website Warning 功能，并使用 REJECT 策略。\n  - DOMAIN,safebrowsing.googleapis.com,DIRECT # 如果您并不信任此服务提供商或防止其下载消耗过多带宽资源，可以进入 Safari 设置，关闭 Fraudulent Website Warning 功能，并使用 REJECT 策略。\n  - DOMAIN,developer.apple.com,Proxy\n  - DOMAIN-SUFFIX,digicert.com,Proxy\n  - DOMAIN,ocsp.apple.com,Proxy\n  - DOMAIN,ocsp.comodoca.com,Proxy\n  - DOMAIN,ocsp.usertrust.com,Proxy\n  - DOMAIN,ocsp.sectigo.com,Proxy\n  - DOMAIN,ocsp.verisign.net,Proxy\n  - DOMAIN-SUFFIX,apple-dns.net,Proxy\n  - DOMAIN,testflight.apple.com,Proxy\n  - DOMAIN,sandbox.itunes.apple.com,Proxy\n  - DOMAIN,itunes.apple.com,Proxy\n  - DOMAIN-SUFFIX,apps.apple.com,Proxy\n  - DOMAIN-SUFFIX,blobstore.apple.com,Proxy\n  - DOMAIN,cvws.icloud-content.com,Proxy\n  - DOMAIN-SUFFIX,mzstatic.com,DIRECT\n  - DOMAIN-SUFFIX,itunes.apple.com,DIRECT\n  - DOMAIN-SUFFIX,icloud.com,DIRECT\n  - DOMAIN-SUFFIX,icloud-content.com,DIRECT\n  - DOMAIN-SUFFIX,me.com,DIRECT\n  - DOMAIN-SUFFIX,aaplimg.com,DIRECT\n  - DOMAIN-SUFFIX,cdn20.com,DIRECT\n  - DOMAIN-SUFFIX,cdn-apple.com,DIRECT\n  - DOMAIN-SUFFIX,akadns.net,DIRECT\n  - DOMAIN-SUFFIX,akamaiedge.net,DIRECT\n  - DOMAIN-SUFFIX,edgekey.net,DIRECT\n  - DOMAIN-SUFFIX,mwcloudcdn.com,DIRECT\n  - DOMAIN-SUFFIX,mwcname.com,DIRECT\n  - DOMAIN-SUFFIX,apple.com,DIRECT\n  - DOMAIN-SUFFIX,apple-cloudkit.com,DIRECT\n  - DOMAIN-SUFFIX,apple-mapkit.com,DIRECT\n  # - DOMAIN,e.crashlytics.com,REJECT //注释此选项有助于大多数App开发者分析崩溃信息；如果您拒绝一切崩溃数据统计、搜集，请取消 # 注释。\n\n  # 国内网站\n  - DOMAIN-SUFFIX,cn,DIRECT\n  - DOMAIN-KEYWORD,-cn,DIRECT\n\n  - DOMAIN-SUFFIX,126.com,DIRECT\n  - DOMAIN-SUFFIX,126.net,DIRECT\n  - DOMAIN-SUFFIX,127.net,DIRECT\n  - DOMAIN-SUFFIX,163.com,DIRECT\n  - DOMAIN-SUFFIX,360buyimg.com,DIRECT\n  - DOMAIN-SUFFIX,36kr.com,DIRECT\n  - DOMAIN-SUFFIX,acfun.tv,DIRECT\n  - DOMAIN-SUFFIX,air-matters.com,DIRECT\n  - DOMAIN-SUFFIX,aixifan.com,DIRECT\n  - DOMAIN-KEYWORD,alicdn,DIRECT\n  - DOMAIN-KEYWORD,alipay,DIRECT\n  - DOMAIN-KEYWORD,taobao,DIRECT\n  - DOMAIN-SUFFIX,amap.com,DIRECT\n  - DOMAIN-SUFFIX,autonavi.com,DIRECT\n  - DOMAIN-KEYWORD,baidu,DIRECT\n  - DOMAIN-SUFFIX,bdimg.com,DIRECT\n  - DOMAIN-SUFFIX,bdstatic.com,DIRECT\n  - DOMAIN-SUFFIX,bilibili.com,DIRECT\n  - DOMAIN-SUFFIX,bilivideo.com,DIRECT\n  - DOMAIN-SUFFIX,caiyunapp.com,DIRECT\n  - DOMAIN-SUFFIX,clouddn.com,DIRECT\n  - DOMAIN-SUFFIX,cnbeta.com,DIRECT\n  - DOMAIN-SUFFIX,cnbetacdn.com,DIRECT\n  - DOMAIN-SUFFIX,cootekservice.com,DIRECT\n  - DOMAIN-SUFFIX,csdn.net,DIRECT\n  - DOMAIN-SUFFIX,ctrip.com,DIRECT\n  - DOMAIN-SUFFIX,dgtle.com,DIRECT\n  - DOMAIN-SUFFIX,dianping.com,DIRECT\n  - DOMAIN-SUFFIX,douban.com,DIRECT\n  - DOMAIN-SUFFIX,doubanio.com,DIRECT\n  - DOMAIN-SUFFIX,duokan.com,DIRECT\n  - DOMAIN-SUFFIX,easou.com,DIRECT\n  - DOMAIN-SUFFIX,ele.me,DIRECT\n  - DOMAIN-SUFFIX,feng.com,DIRECT\n  - DOMAIN-SUFFIX,fir.im,DIRECT\n  - DOMAIN-SUFFIX,frdic.com,DIRECT\n  - DOMAIN-SUFFIX,g-cores.com,DIRECT\n  - DOMAIN-SUFFIX,godic.net,DIRECT\n  - DOMAIN-SUFFIX,gtimg.com,DIRECT\n  - DOMAIN,cdn.hockeyapp.net,DIRECT\n  - DOMAIN-SUFFIX,hongxiu.com,DIRECT\n  - DOMAIN-SUFFIX,hxcdn.net,DIRECT\n  - DOMAIN-SUFFIX,iciba.com,DIRECT\n  - DOMAIN-SUFFIX,ifeng.com,DIRECT\n  - DOMAIN-SUFFIX,ifengimg.com,DIRECT\n  - DOMAIN-SUFFIX,ipip.net,DIRECT\n  - DOMAIN-SUFFIX,iqiyi.com,DIRECT\n  - DOMAIN-SUFFIX,jd.com,DIRECT\n  - DOMAIN-SUFFIX,jianshu.com,DIRECT\n  - DOMAIN-SUFFIX,knewone.com,DIRECT\n  - DOMAIN-SUFFIX,le.com,DIRECT\n  - DOMAIN-SUFFIX,lecloud.com,DIRECT\n  - DOMAIN-SUFFIX,lemicp.com,DIRECT\n  - DOMAIN-SUFFIX,licdn.com,DIRECT\n  - DOMAIN-SUFFIX,luoo.net,DIRECT\n  - DOMAIN-SUFFIX,meituan.com,DIRECT\n  - DOMAIN-SUFFIX,meituan.net,DIRECT\n  - DOMAIN-SUFFIX,mi.com,DIRECT\n  - DOMAIN-SUFFIX,miaopai.com,DIRECT\n  - DOMAIN-SUFFIX,microsoft.com,DIRECT\n  - DOMAIN-SUFFIX,microsoftonline.com,DIRECT\n  - DOMAIN-SUFFIX,miui.com,DIRECT\n  - DOMAIN-SUFFIX,miwifi.com,DIRECT\n  - DOMAIN-SUFFIX,mob.com,DIRECT\n  - DOMAIN-SUFFIX,netease.com,DIRECT\n  - DOMAIN-SUFFIX,office.com,DIRECT\n  - DOMAIN-SUFFIX,office365.com,DIRECT\n  - DOMAIN-KEYWORD,officecdn,DIRECT\n  - DOMAIN-SUFFIX,oschina.net,DIRECT\n  - DOMAIN-SUFFIX,ppsimg.com,DIRECT\n  - DOMAIN-SUFFIX,pstatp.com,DIRECT\n  - DOMAIN-SUFFIX,qcloud.com,DIRECT\n  - DOMAIN-SUFFIX,qdaily.com,DIRECT\n  - DOMAIN-SUFFIX,qdmm.com,DIRECT\n  - DOMAIN-SUFFIX,qhimg.com,DIRECT\n  - DOMAIN-SUFFIX,qhres.com,DIRECT\n  - DOMAIN-SUFFIX,qidian.com,DIRECT\n  - DOMAIN-SUFFIX,qihucdn.com,DIRECT\n  - DOMAIN-SUFFIX,qiniu.com,DIRECT\n  - DOMAIN-SUFFIX,qiniucdn.com,DIRECT\n  - DOMAIN-SUFFIX,qiyipic.com,DIRECT\n  - DOMAIN-SUFFIX,qq.com,DIRECT\n  - DOMAIN-SUFFIX,qqurl.com,DIRECT\n  - DOMAIN-SUFFIX,rarbg.to,DIRECT\n  - DOMAIN-SUFFIX,ruguoapp.com,DIRECT\n  - DOMAIN-SUFFIX,segmentfault.com,DIRECT\n  - DOMAIN-SUFFIX,sinaapp.com,DIRECT\n  - DOMAIN-SUFFIX,smzdm.com,DIRECT\n  - DOMAIN-SUFFIX,snapdrop.net,DIRECT\n  - DOMAIN-SUFFIX,sogou.com,DIRECT\n  - DOMAIN-SUFFIX,sogoucdn.com,DIRECT\n  - DOMAIN-SUFFIX,sohu.com,DIRECT\n  - DOMAIN-SUFFIX,soku.com,DIRECT\n  - DOMAIN-SUFFIX,speedtest.net,DIRECT\n  - DOMAIN-SUFFIX,sspai.com,DIRECT\n  - DOMAIN-SUFFIX,suning.com,DIRECT\n  - DOMAIN-SUFFIX,taobao.com,DIRECT\n  - DOMAIN-SUFFIX,tencent.com,DIRECT\n  - DOMAIN-SUFFIX,tenpay.com,DIRECT\n  - DOMAIN-SUFFIX,tianyancha.com,DIRECT\n  - DOMAIN-SUFFIX,tmall.com,DIRECT\n  - DOMAIN-SUFFIX,tudou.com,DIRECT\n  - DOMAIN-SUFFIX,umetrip.com,DIRECT\n  - DOMAIN-SUFFIX,upaiyun.com,DIRECT\n  - DOMAIN-SUFFIX,upyun.com,DIRECT\n  - DOMAIN-SUFFIX,veryzhun.com,DIRECT\n  - DOMAIN-SUFFIX,weather.com,DIRECT\n  - DOMAIN-SUFFIX,weibo.com,DIRECT\n  - DOMAIN-SUFFIX,xiami.com,DIRECT\n  - DOMAIN-SUFFIX,xiami.net,DIRECT\n  - DOMAIN-SUFFIX,xiaomicp.com,DIRECT\n  - DOMAIN-SUFFIX,ximalaya.com,DIRECT\n  - DOMAIN-SUFFIX,xmcdn.com,DIRECT\n  - DOMAIN-SUFFIX,xunlei.com,DIRECT\n  - DOMAIN-SUFFIX,yhd.com,DIRECT\n  - DOMAIN-SUFFIX,yihaodianimg.com,DIRECT\n  - DOMAIN-SUFFIX,yinxiang.com,DIRECT\n  - DOMAIN-SUFFIX,ykimg.com,DIRECT\n  - DOMAIN-SUFFIX,youdao.com,DIRECT\n  - DOMAIN-SUFFIX,youku.com,DIRECT\n  - DOMAIN-SUFFIX,zealer.com,DIRECT\n  - DOMAIN-SUFFIX,zhihu.com,DIRECT\n  - DOMAIN-SUFFIX,zhimg.com,DIRECT\n  - DOMAIN-SUFFIX,zimuzu.tv,DIRECT\n  - DOMAIN-SUFFIX,zoho.com,DIRECT\n\n  # 抗 DNS 污染\n  - DOMAIN-KEYWORD,amazon,Proxy\n  - DOMAIN-KEYWORD,google,Proxy\n  - DOMAIN-KEYWORD,gmail,Proxy\n  - DOMAIN-KEYWORD,youtube,Proxy\n  - DOMAIN-KEYWORD,facebook,Proxy\n  - DOMAIN-SUFFIX,fb.me,Proxy\n  - DOMAIN-SUFFIX,fbcdn.net,Proxy\n  - DOMAIN-KEYWORD,twitter,Proxy\n  - DOMAIN-KEYWORD,instagram,Proxy\n  - DOMAIN-KEYWORD,dropbox,Proxy\n  - DOMAIN-SUFFIX,twimg.com,Proxy\n  - DOMAIN-KEYWORD,blogspot,Proxy\n  - DOMAIN-SUFFIX,youtu.be,Proxy\n  - DOMAIN-KEYWORD,whatsapp,Proxy\n\n  # 常见广告域名屏蔽\n  - DOMAIN-KEYWORD,admarvel,REJECT\n  - DOMAIN-KEYWORD,admaster,REJECT\n  - DOMAIN-KEYWORD,adsage,REJECT\n  - DOMAIN-KEYWORD,adsmogo,REJECT\n  - DOMAIN-KEYWORD,adsrvmedia,REJECT\n  - DOMAIN-KEYWORD,adwords,REJECT\n  - DOMAIN-KEYWORD,adservice,REJECT\n  - DOMAIN-SUFFIX,appsflyer.com,REJECT\n  - DOMAIN-KEYWORD,domob,REJECT\n  - DOMAIN-SUFFIX,doubleclick.net,REJECT\n  - DOMAIN-KEYWORD,duomeng,REJECT\n  - DOMAIN-KEYWORD,dwtrack,REJECT\n  - DOMAIN-KEYWORD,guanggao,REJECT\n  - DOMAIN-KEYWORD,lianmeng,REJECT\n  - DOMAIN-SUFFIX,mmstat.com,REJECT\n  - DOMAIN-KEYWORD,mopub,REJECT\n  - DOMAIN-KEYWORD,omgmta,REJECT\n  - DOMAIN-KEYWORD,openx,REJECT\n  - DOMAIN-KEYWORD,partnerad,REJECT\n  - DOMAIN-KEYWORD,pingfore,REJECT\n  - DOMAIN-KEYWORD,supersonicads,REJECT\n  - DOMAIN-KEYWORD,uedas,REJECT\n  - DOMAIN-KEYWORD,umeng,REJECT\n  - DOMAIN-KEYWORD,usage,REJECT\n  - DOMAIN-SUFFIX,vungle.com,REJECT\n  - DOMAIN-KEYWORD,wlmonitor,REJECT\n  - DOMAIN-KEYWORD,zjtoolbar,REJECT\n\n  # 国外网站\n  - DOMAIN-SUFFIX,9to5mac.com,Proxy\n  - DOMAIN-SUFFIX,abpchina.org,Proxy\n  - DOMAIN-SUFFIX,adblockplus.org,Proxy\n  - DOMAIN-SUFFIX,adobe.com,Proxy\n  - DOMAIN-SUFFIX,akamaized.net,Proxy\n  - DOMAIN-SUFFIX,alfredapp.com,Proxy\n  - DOMAIN-SUFFIX,amplitude.com,Proxy\n  - DOMAIN-SUFFIX,ampproject.org,Proxy\n  - DOMAIN-SUFFIX,android.com,Proxy\n  - DOMAIN-SUFFIX,angularjs.org,Proxy\n  - DOMAIN-SUFFIX,aolcdn.com,Proxy\n  - DOMAIN-SUFFIX,apkpure.com,Proxy\n  - DOMAIN-SUFFIX,appledaily.com,Proxy\n  - DOMAIN-SUFFIX,appshopper.com,Proxy\n  - DOMAIN-SUFFIX,appspot.com,Proxy\n  - DOMAIN-SUFFIX,arcgis.com,Proxy\n  - DOMAIN-SUFFIX,archive.org,Proxy\n  - DOMAIN-SUFFIX,armorgames.com,Proxy\n  - DOMAIN-SUFFIX,aspnetcdn.com,Proxy\n  - DOMAIN-SUFFIX,att.com,Proxy\n  - DOMAIN-SUFFIX,awsstatic.com,Proxy\n  - DOMAIN-SUFFIX,azureedge.net,Proxy\n  - DOMAIN-SUFFIX,azurewebsites.net,Proxy\n  - DOMAIN-SUFFIX,bing.com,Proxy\n  - DOMAIN-SUFFIX,bintray.com,Proxy\n  - DOMAIN-SUFFIX,bit.com,Proxy\n  - DOMAIN-SUFFIX,bit.ly,Proxy\n  - DOMAIN-SUFFIX,bitbucket.org,Proxy\n  - DOMAIN-SUFFIX,bjango.com,Proxy\n  - DOMAIN-SUFFIX,bkrtx.com,Proxy\n  - DOMAIN-SUFFIX,blog.com,Proxy\n  - DOMAIN-SUFFIX,blogcdn.com,Proxy\n  - DOMAIN-SUFFIX,blogger.com,Proxy\n  - DOMAIN-SUFFIX,blogsmithmedia.com,Proxy\n  - DOMAIN-SUFFIX,blogspot.com,Proxy\n  - DOMAIN-SUFFIX,blogspot.hk,Proxy\n  - DOMAIN-SUFFIX,bloomberg.com,Proxy\n  - DOMAIN-SUFFIX,box.com,Proxy\n  - DOMAIN-SUFFIX,box.net,Proxy\n  - DOMAIN-SUFFIX,cachefly.net,Proxy\n  - DOMAIN-SUFFIX,chromium.org,Proxy\n  - DOMAIN-SUFFIX,cl.ly,Proxy\n  - DOMAIN-SUFFIX,cloudflare.com,Proxy\n  - DOMAIN-SUFFIX,cloudfront.net,Proxy\n  - DOMAIN-SUFFIX,cloudmagic.com,Proxy\n  - DOMAIN-SUFFIX,cmail19.com,Proxy\n  - DOMAIN-SUFFIX,cnet.com,Proxy\n  - DOMAIN-SUFFIX,cocoapods.org,Proxy\n  - DOMAIN-SUFFIX,comodoca.com,Proxy\n  - DOMAIN-SUFFIX,crashlytics.com,Proxy\n  - DOMAIN-SUFFIX,culturedcode.com,Proxy\n  - DOMAIN-SUFFIX,d.pr,Proxy\n  - DOMAIN-SUFFIX,danilo.to,Proxy\n  - DOMAIN-SUFFIX,dayone.me,Proxy\n  - DOMAIN-SUFFIX,db.tt,Proxy\n  - DOMAIN-SUFFIX,deskconnect.com,Proxy\n  - DOMAIN-SUFFIX,disq.us,Proxy\n  - DOMAIN-SUFFIX,disqus.com,Proxy\n  - DOMAIN-SUFFIX,disquscdn.com,Proxy\n  - DOMAIN-SUFFIX,dnsimple.com,Proxy\n  - DOMAIN-SUFFIX,docker.com,Proxy\n  - DOMAIN-SUFFIX,dribbble.com,Proxy\n  - DOMAIN-SUFFIX,droplr.com,Proxy\n  - DOMAIN-SUFFIX,duckduckgo.com,Proxy\n  - DOMAIN-SUFFIX,dueapp.com,Proxy\n  - DOMAIN-SUFFIX,dytt8.net,Proxy\n  - DOMAIN-SUFFIX,edgecastcdn.net,Proxy\n  - DOMAIN-SUFFIX,edgekey.net,Proxy\n  - DOMAIN-SUFFIX,edgesuite.net,Proxy\n  - DOMAIN-SUFFIX,engadget.com,Proxy\n  - DOMAIN-SUFFIX,entrust.net,Proxy\n  - DOMAIN-SUFFIX,eurekavpt.com,Proxy\n  - DOMAIN-SUFFIX,evernote.com,Proxy\n  - DOMAIN-SUFFIX,fabric.io,Proxy\n  - DOMAIN-SUFFIX,fast.com,Proxy\n  - DOMAIN-SUFFIX,fastly.net,Proxy\n  - DOMAIN-SUFFIX,fc2.com,Proxy\n  - DOMAIN-SUFFIX,feedburner.com,Proxy\n  - DOMAIN-SUFFIX,feedly.com,Proxy\n  - DOMAIN-SUFFIX,feedsportal.com,Proxy\n  - DOMAIN-SUFFIX,fiftythree.com,Proxy\n  - DOMAIN-SUFFIX,firebaseio.com,Proxy\n  - DOMAIN-SUFFIX,flexibits.com,Proxy\n  - DOMAIN-SUFFIX,flickr.com,Proxy\n  - DOMAIN-SUFFIX,flipboard.com,Proxy\n  - DOMAIN-SUFFIX,g.co,Proxy\n  - DOMAIN-SUFFIX,gabia.net,Proxy\n  - DOMAIN-SUFFIX,geni.us,Proxy\n  - DOMAIN-SUFFIX,gfx.ms,Proxy\n  - DOMAIN-SUFFIX,ggpht.com,Proxy\n  - DOMAIN-SUFFIX,ghostnoteapp.com,Proxy\n  - DOMAIN-SUFFIX,git.io,Proxy\n  - DOMAIN-KEYWORD,github,Proxy\n  - DOMAIN-SUFFIX,globalsign.com,Proxy\n  - DOMAIN-SUFFIX,gmodules.com,Proxy\n  - DOMAIN-SUFFIX,godaddy.com,Proxy\n  - DOMAIN-SUFFIX,golang.org,Proxy\n  - DOMAIN-SUFFIX,gongm.in,Proxy\n  - DOMAIN-SUFFIX,goo.gl,Proxy\n  - DOMAIN-SUFFIX,goodreaders.com,Proxy\n  - DOMAIN-SUFFIX,goodreads.com,Proxy\n  - DOMAIN-SUFFIX,gravatar.com,Proxy\n  - DOMAIN-SUFFIX,gstatic.com,Proxy\n  - DOMAIN-SUFFIX,gvt0.com,Proxy\n  - DOMAIN-SUFFIX,hockeyapp.net,Proxy\n  - DOMAIN-SUFFIX,hotmail.com,Proxy\n  - DOMAIN-SUFFIX,icons8.com,Proxy\n  - DOMAIN-SUFFIX,ifixit.com,Proxy\n  - DOMAIN-SUFFIX,ift.tt,Proxy\n  - DOMAIN-SUFFIX,ifttt.com,Proxy\n  - DOMAIN-SUFFIX,iherb.com,Proxy\n  - DOMAIN-SUFFIX,imageshack.us,Proxy\n  - DOMAIN-SUFFIX,img.ly,Proxy\n  - DOMAIN-SUFFIX,imgur.com,Proxy\n  - DOMAIN-SUFFIX,imore.com,Proxy\n  - DOMAIN-SUFFIX,instapaper.com,Proxy\n  - DOMAIN-SUFFIX,ipn.li,Proxy\n  - DOMAIN-SUFFIX,is.gd,Proxy\n  - DOMAIN-SUFFIX,issuu.com,Proxy\n  - DOMAIN-SUFFIX,itgonglun.com,Proxy\n  - DOMAIN-SUFFIX,itun.es,Proxy\n  - DOMAIN-SUFFIX,ixquick.com,Proxy\n  - DOMAIN-SUFFIX,j.mp,Proxy\n  - DOMAIN-SUFFIX,js.revsci.net,Proxy\n  - DOMAIN-SUFFIX,jshint.com,Proxy\n  - DOMAIN-SUFFIX,jtvnw.net,Proxy\n  - DOMAIN-SUFFIX,justgetflux.com,Proxy\n  - DOMAIN-SUFFIX,kat.cr,Proxy\n  - DOMAIN-SUFFIX,klip.me,Proxy\n  - DOMAIN-SUFFIX,libsyn.com,Proxy\n  - DOMAIN-SUFFIX,linkedin.com,Proxy\n  - DOMAIN-SUFFIX,linode.com,Proxy\n  - DOMAIN-SUFFIX,lithium.com,Proxy\n  - DOMAIN-SUFFIX,littlehj.com,Proxy\n  - DOMAIN-SUFFIX,live.com,Proxy\n  - DOMAIN-SUFFIX,live.net,Proxy\n  - DOMAIN-SUFFIX,livefilestore.com,Proxy\n  - DOMAIN-SUFFIX,llnwd.net,Proxy\n  - DOMAIN-SUFFIX,macid.co,Proxy\n  - DOMAIN-SUFFIX,macromedia.com,Proxy\n  - DOMAIN-SUFFIX,macrumors.com,Proxy\n  - DOMAIN-SUFFIX,mashable.com,Proxy\n  - DOMAIN-SUFFIX,mathjax.org,Proxy\n  - DOMAIN-SUFFIX,medium.com,Proxy\n  - DOMAIN-SUFFIX,mega.co.nz,Proxy\n  - DOMAIN-SUFFIX,mega.nz,Proxy\n  - DOMAIN-SUFFIX,megaupload.com,Proxy\n  - DOMAIN-SUFFIX,microsofttranslator.com,Proxy\n  - DOMAIN-SUFFIX,mindnode.com,Proxy\n  - DOMAIN-SUFFIX,mobile01.com,Proxy\n  - DOMAIN-SUFFIX,modmyi.com,Proxy\n  - DOMAIN-SUFFIX,msedge.net,Proxy\n  - DOMAIN-SUFFIX,myfontastic.com,Proxy\n  - DOMAIN-SUFFIX,name.com,Proxy\n  - DOMAIN-SUFFIX,nextmedia.com,Proxy\n  - DOMAIN-SUFFIX,nsstatic.net,Proxy\n  - DOMAIN-SUFFIX,nssurge.com,Proxy\n  - DOMAIN-SUFFIX,nyt.com,Proxy\n  - DOMAIN-SUFFIX,nytimes.com,Proxy\n  - DOMAIN-SUFFIX,omnigroup.com,Proxy\n  - DOMAIN-SUFFIX,onedrive.com,Proxy\n  - DOMAIN-SUFFIX,onenote.com,Proxy\n  - DOMAIN-SUFFIX,ooyala.com,Proxy\n  - DOMAIN-SUFFIX,openvpn.net,Proxy\n  - DOMAIN-SUFFIX,openwrt.org,Proxy\n  - DOMAIN-SUFFIX,orkut.com,Proxy\n  - DOMAIN-SUFFIX,osxdaily.com,Proxy\n  - DOMAIN-SUFFIX,outlook.com,Proxy\n  - DOMAIN-SUFFIX,ow.ly,Proxy\n  - DOMAIN-SUFFIX,paddleapi.com,Proxy\n  - DOMAIN-SUFFIX,parallels.com,Proxy\n  - DOMAIN-SUFFIX,parse.com,Proxy\n  - DOMAIN-SUFFIX,pdfexpert.com,Proxy\n  - DOMAIN-SUFFIX,periscope.tv,Proxy\n  - DOMAIN-SUFFIX,pinboard.in,Proxy\n  - DOMAIN-SUFFIX,pinterest.com,Proxy\n  - DOMAIN-SUFFIX,pixelmator.com,Proxy\n  - DOMAIN-SUFFIX,pixiv.net,Proxy\n  - DOMAIN-SUFFIX,playpcesor.com,Proxy\n  - DOMAIN-SUFFIX,playstation.com,Proxy\n  - DOMAIN-SUFFIX,playstation.com.hk,Proxy\n  - DOMAIN-SUFFIX,playstation.net,Proxy\n  - DOMAIN-SUFFIX,playstationnetwork.com,Proxy\n  - DOMAIN-SUFFIX,pushwoosh.com,Proxy\n  - DOMAIN-SUFFIX,rime.im,Proxy\n  - DOMAIN-SUFFIX,servebom.com,Proxy\n  - DOMAIN-SUFFIX,sfx.ms,Proxy\n  - DOMAIN-SUFFIX,shadowsocks.org,Proxy\n  - DOMAIN-SUFFIX,sharethis.com,Proxy\n  - DOMAIN-SUFFIX,shazam.com,Proxy\n  - DOMAIN-SUFFIX,skype.com,Proxy\n  - DOMAIN-SUFFIX,smartdnsProxy.com,Proxy\n  - DOMAIN-SUFFIX,smartmailcloud.com,Proxy\n  - DOMAIN-SUFFIX,sndcdn.com,Proxy\n  - DOMAIN-SUFFIX,sony.com,Proxy\n  - DOMAIN-SUFFIX,soundcloud.com,Proxy\n  - DOMAIN-SUFFIX,sourceforge.net,Proxy\n  - DOMAIN-SUFFIX,spotify.com,Proxy\n  - DOMAIN-SUFFIX,squarespace.com,Proxy\n  - DOMAIN-SUFFIX,sstatic.net,Proxy\n  - DOMAIN-SUFFIX,st.luluku.pw,Proxy\n  - DOMAIN-SUFFIX,stackoverflow.com,Proxy\n  - DOMAIN-SUFFIX,startpage.com,Proxy\n  - DOMAIN-SUFFIX,staticflickr.com,Proxy\n  - DOMAIN-SUFFIX,steamcommunity.com,Proxy\n  - DOMAIN-SUFFIX,symauth.com,Proxy\n  - DOMAIN-SUFFIX,symcb.com,Proxy\n  - DOMAIN-SUFFIX,symcd.com,Proxy\n  - DOMAIN-SUFFIX,tapbots.com,Proxy\n  - DOMAIN-SUFFIX,tapbots.net,Proxy\n  - DOMAIN-SUFFIX,tdesktop.com,Proxy\n  - DOMAIN-SUFFIX,techcrunch.com,Proxy\n  - DOMAIN-SUFFIX,techsmith.com,Proxy\n  - DOMAIN-SUFFIX,thepiratebay.org,Proxy\n  - DOMAIN-SUFFIX,theverge.com,Proxy\n  - DOMAIN-SUFFIX,time.com,Proxy\n  - DOMAIN-SUFFIX,timeinc.net,Proxy\n  - DOMAIN-SUFFIX,tiny.cc,Proxy\n  - DOMAIN-SUFFIX,tinypic.com,Proxy\n  - DOMAIN-SUFFIX,tmblr.co,Proxy\n  - DOMAIN-SUFFIX,todoist.com,Proxy\n  - DOMAIN-SUFFIX,trello.com,Proxy\n  - DOMAIN-SUFFIX,trustasiassl.com,Proxy\n  - DOMAIN-SUFFIX,tumblr.co,Proxy\n  - DOMAIN-SUFFIX,tumblr.com,Proxy\n  - DOMAIN-SUFFIX,tweetdeck.com,Proxy\n  - DOMAIN-SUFFIX,tweetmarker.net,Proxy\n  - DOMAIN-SUFFIX,twitch.tv,Proxy\n  - DOMAIN-SUFFIX,txmblr.com,Proxy\n  - DOMAIN-SUFFIX,typekit.net,Proxy\n  - DOMAIN-SUFFIX,ubertags.com,Proxy\n  - DOMAIN-SUFFIX,ublock.org,Proxy\n  - DOMAIN-SUFFIX,ubnt.com,Proxy\n  - DOMAIN-SUFFIX,ulyssesapp.com,Proxy\n  - DOMAIN-SUFFIX,urchin.com,Proxy\n  - DOMAIN-SUFFIX,usertrust.com,Proxy\n  - DOMAIN-SUFFIX,v.gd,Proxy\n  - DOMAIN-SUFFIX,v2ex.com,Proxy\n  - DOMAIN-SUFFIX,vimeo.com,Proxy\n  - DOMAIN-SUFFIX,vimeocdn.com,Proxy\n  - DOMAIN-SUFFIX,vine.co,Proxy\n  - DOMAIN-SUFFIX,vivaldi.com,Proxy\n  - DOMAIN-SUFFIX,vox-cdn.com,Proxy\n  - DOMAIN-SUFFIX,vsco.co,Proxy\n  - DOMAIN-SUFFIX,vultr.com,Proxy\n  - DOMAIN-SUFFIX,w.org,Proxy\n  - DOMAIN-SUFFIX,w3schools.com,Proxy\n  - DOMAIN-SUFFIX,webtype.com,Proxy\n  - DOMAIN-SUFFIX,wikiwand.com,Proxy\n  - DOMAIN-SUFFIX,wikileaks.org,Proxy\n  - DOMAIN-SUFFIX,wikimedia.org,Proxy\n  - DOMAIN-SUFFIX,wikipedia.com,Proxy\n  - DOMAIN-SUFFIX,wikipedia.org,Proxy\n  - DOMAIN-SUFFIX,windows.com,Proxy\n  - DOMAIN-SUFFIX,windows.net,Proxy\n  - DOMAIN-SUFFIX,wire.com,Proxy\n  - DOMAIN-SUFFIX,wordpress.com,Proxy\n  - DOMAIN-SUFFIX,workflowy.com,Proxy\n  - DOMAIN-SUFFIX,wp.com,Proxy\n  - DOMAIN-SUFFIX,wsj.com,Proxy\n  - DOMAIN-SUFFIX,wsj.net,Proxy\n  - DOMAIN-SUFFIX,xda-developers.com,Proxy\n  - DOMAIN-SUFFIX,xeeno.com,Proxy\n  - DOMAIN-SUFFIX,xiti.com,Proxy\n  - DOMAIN-SUFFIX,yahoo.com,Proxy\n  - DOMAIN-SUFFIX,yimg.com,Proxy\n  - DOMAIN-SUFFIX,ying.com,Proxy\n  - DOMAIN-SUFFIX,yoyo.org,Proxy\n  - DOMAIN-SUFFIX,ytimg.com,Proxy\n\n  # Telegram\n  - DOMAIN-SUFFIX,telegra.ph,Proxy\n  - DOMAIN-SUFFIX,telegram.org,Proxy\n  - IP-CIDR,91.108.4.0/22,Proxy\n  - IP-CIDR,91.108.8.0/21,Proxy\n  - IP-CIDR,91.108.16.0/22,Proxy\n  - IP-CIDR,91.108.56.0/22,Proxy\n  - IP-CIDR,149.154.160.0/20,Proxy\n  - IP-CIDR6,2001:67c:4e8::/48,Proxy\n  - IP-CIDR6,2001:b28:f23d::/48,Proxy\n  - IP-CIDR6,2001:b28:f23f::/48,Proxy\n\n  # LAN\n  - DOMAIN,injections.adguard.org,DIRECT\n  - DOMAIN,local.adguard.org,DIRECT\n  - DOMAIN-SUFFIX,local,DIRECT\n  - IP-CIDR,127.0.0.0/8,DIRECT\n  - IP-CIDR,172.16.0.0/12,DIRECT\n  - IP-CIDR,192.168.0.0/16,DIRECT\n  - IP-CIDR,10.0.0.0/8,DIRECT\n  - IP-CIDR,17.0.0.0/8,DIRECT\n  - IP-CIDR,100.64.0.0/10,DIRECT\n  - IP-CIDR,224.0.0.0/4,DIRECT\n  - IP-CIDR6,fe80::/10,DIRECT\n\n  # 最终规则\n  - GEOIP,CN,DIRECT\n  - MATCH,Proxy")
	baseRuleMap := make(map[string]interface{})
	unmarshalReadyToWriteRuleErr := yaml.Unmarshal(baseRuleBody, &baseRuleMap)
	if unmarshalReadyToWriteRuleErr != nil {
		log.Printf("error: %v\n", unmarshalReadyToWriteRuleErr)
		log.Println("解析基础规则失败，请检查 url 是否正确 ！！！")
		return nil, nil, true
	}
	return userConfigMap, baseRuleMap, false
}

// 获取配置文件，二选一的，优先返回用户设置的，其次返回基础配置的，最后返回空
func getConfigFieldValue(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) interface{} {
	if userConfigMap[fieldName] != nil {
		return userConfigMap[fieldName]
	} else if baseRuleConfigMap[fieldName] != nil {
		return baseRuleConfigMap[fieldName]
	} else {
		return nil
	}
	return nil
}

// 把用户自定义设置和基础配置合并后返回
func getConfigFieldMergeValueArr(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) []interface{} {
	var valueSlice []interface{}
	if userConfigMap[fieldName] != nil {
		valueSlice = append(valueSlice, userConfigMap[fieldName].([]interface{})...)
	}
	if baseRuleConfigMap[fieldName] != nil {
		valueSlice = append(valueSlice, baseRuleConfigMap[fieldName].([]interface{})...)
	}
	return valueSlice
}

func getConfigFieldMergeValueMap(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) map[interface{}]interface{} {
	valueMap := make(map[interface{}]interface{})
	if baseRuleConfigMap[fieldName] != nil {
		baseRuleConfigValueMap := baseRuleConfigMap[fieldName].(map[interface{}]interface{})
		for key := range baseRuleConfigValueMap {
			valueMap[key] = baseRuleConfigValueMap[key]
		}
	}
	if userConfigMap[fieldName] != nil {
		userConfigValueMap := userConfigMap[fieldName].(map[interface{}]interface{})
		for key := range userConfigValueMap {
			valueMap[key] = userConfigValueMap[key]
		}
	}
	return valueMap
}

// 生成 proxyGroup 并且返回该组的 proxy 名称
func generateGroupAndProxyNameArr(base64ProxyServerArr []map[interface{}]interface{}, proxySourceName interface{}) (map[interface{}]interface{}, []interface{}) {
	var tempProxyNameArr []interface{}
	for mapIndex := range base64ProxyServerArr {
		tempProxyNameArr = append(tempProxyNameArr, base64ProxyServerArr[mapIndex]["name"])
	}
	// 构造一个 group
	proxyGroupMap := make(map[interface{}]interface{})
	proxyGroupMap["name"] = proxySourceName
	proxyGroupMap["type"] = "select"
	proxyGroupMap["proxies"] = tempProxyNameArr

	return proxyGroupMap, tempProxyNameArr
}

// 把 ProxyName数组 插入到 group 中
func generateProxyNameToGroup(proxyGroups interface{}, proxyNameArr []interface{}, filterProxyGroupArr []interface{}) []map[interface{}]interface{} {
	// 遍历 判定是否包含use 如果没有就插入 proxies
	var outputProxyGroupMap []map[interface{}]interface{}
filterStart:
	for _, proxyGroup := range proxyGroups.([]interface{}) {
		proxyGroupMap := proxyGroup.(map[interface{}]interface{})
		for _, filterName := range filterProxyGroupArr {
			if filterName == proxyGroupMap["name"] {
				continue filterStart
			}
		}

		if proxyGroupMap["use"] == nil {
			proxyGroupMap["proxies"] = proxyNameArr
		}

		outputProxyGroupMap = append(outputProxyGroupMap, proxyGroupMap)
	}
	return outputProxyGroupMap
}
