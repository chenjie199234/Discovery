# Discovery
该服务为微服务注册中心</br>
group为default</br>
name为discovery</br>
监听端口为10000</br>
web监听端口为8000</br>
/infos 获取所有的注册服务信息</br>
/info/{group}/{name} 获取{group}下的{name}的注册信息</br>
# 环境变量
SERVER_VERIFY_DATA 	连接注册中心时需要用的验证数据</br>
RUN_ENV 		运行环境,如:test,pre,prod</br>
DEPLOY_ENV 		部署环境,如:host,k8s</br>
