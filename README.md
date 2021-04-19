# Discovery
	该服务为微服务注册中心,自身group为default,自身name为discovery
	注册中心监听端口为10000
	web监听端口为8000
	/infos 获取所有的注册服务信息
	/info/{group}/{name} 获取{group}下的{name}的注册信息
# 原理
	1.基于stream的tcpsocket的简易注册中心,客户端向所有的discoveryserver监听关注的信息然后将信息在客户端做整合
	2.每个client都会维持于所有discoveryserver的tcp长链接,因此节点变动时能实时更新,节点变动时信息的同步延迟小
# 环境变量
	SERVER_VERIFY_DATA 	连接该注册中心时需要用的验证数据
	RUN_ENV 		运行环境,如:test,pre,prod
	DEPLOY_ENV 		部署环境,如:host,k8s
