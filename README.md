# cdk-go-example
CDK Golang 示例

说明: 在CDK中分为L2, L1 construct , L2是高级抽象，L1是Cloudformation映射，以vpc为例, L2 : ec2.NewVpc(), L1: ec2.NewCfnVpc(), CDK 目前v1版本已经停止支持了，所以我们使用的都是v2版本, 但是v2版本里面有些组件不支持L2模块，比如emr, elasticache, msk(有alpha版本）。

本项目在一个stack里面创建了VPC(subnet, natway), RDS(Aurora),ElastiCache(Redis,cluster model), EMR(Flink, Spark), MSK, 整个resource创建大概要40分钟，主要是MSK, RDS时间较长。

1. 安装CDK,并克隆本项目

```bash
npm install -g aws-cdk
mkdir saas && cd saas

#如果需要初始化自己的项 cdk init app --language=go

#克隆example
git clone https://github.com/stevensu1977/cdk-go-example
```

2. 部署

```bash
#输出cloudformation 文件
cdk synth 

#部署
cdk deploy

#清除资源
cdk destroy

```

