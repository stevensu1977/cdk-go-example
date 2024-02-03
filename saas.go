package main

import (
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	cdk "github.com/aws/aws-cdk-go/awscdk/v2"

	ec2 "github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	msk "github.com/aws/aws-cdk-go/awscdkmskalpha/v2"

	elasticache "github.com/aws/aws-cdk-go/awscdk/v2/awselasticache"
	rds "github.com/aws/aws-cdk-go/awscdk/v2/awsrds"

	emr "github.com/aws/aws-cdk-go/awscdk/v2/awsemr"
	iam "github.com/aws/aws-cdk-go/awscdk/v2/awsiam"

	secretmgr "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
)

type SaasStackProps struct {
	awscdk.StackProps
}

const (
	PublicSubnet  = ec2.SubnetType_PUBLIC
	PrivateSubnet = ec2.SubnetType_PRIVATE_WITH_EGRESS
)

//快速返回子网配置
func subnetConfig(name string, subnetType ec2.SubnetType) *ec2.SubnetConfiguration {
	return &ec2.SubnetConfiguration{
		SubnetType: subnetType,
		Name:       aws.String(name),
		CidrMask:   aws.Float64(24),
	}
}

func NewSaasStack(scope constructs.Construct, id string, props *SaasStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	stack := NewSaasStack(app, "SaasStack", &SaasStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	rdsSubnet01 := ec2.SubnetConfiguration{
		SubnetType: ec2.SubnetType_PRIVATE_ISOLATED,
		Name:       aws.String("rds-az1"),
		CidrMask:   aws.Float64(28),
	}

	// 创建VPC
	vpc := ec2.NewVpc(stack, aws.String("VPC"), &ec2.VpcProps{
		//Cidr:        aws.String("172.30.0.0/16"),
		AvailabilityZones: &[]*string{
			aws.String("us-east-1a"),
			aws.String("us-east-1b"),
			aws.String("us-east-1c"),
		},
		IpAddresses: ec2.IpAddresses_Cidr(aws.String("172.31.0.0/16")),
		//MaxAzs:      aws.Float64(3),
		NatGateways: aws.Float64(1),
		SubnetConfiguration: &[]*ec2.SubnetConfiguration{
			subnetConfig("PublicSubnet", PublicSubnet),

			subnetConfig("PrivateSubnet", PrivateSubnet),
			&rdsSubnet01,
		},
	})

	//创建EC2 安全组
	ec2SecurityGroup := ec2.NewSecurityGroup(stack, aws.String("saas-ec2-security-group"), &ec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: aws.Bool(true),
	})
	ec2SecurityGroup.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("172.31.0.0/16")), ec2.Port_AllTraffic(), aws.String("allow from vpc ec2 "))

	//创建WebServer 安全组,开启了公网80,22端口
	webServerSecurityGroup := ec2.NewSecurityGroup(stack, aws.String("saas-webserver-security-group"), &ec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: aws.Bool(true),
	})

	webServerSecurityGroup.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("0.0.0.0/0")), ec2.Port_Tcp(aws.Float64(80)), aws.String("allow from webserver "))
	webServerSecurityGroup.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("0.0.0.0/0")), ec2.Port_Tcp(aws.Float64(22)), aws.String("allow from webserver "))

	//创建RDS 安全组
	rdsSecurityGroup := ec2.NewSecurityGroup(stack, aws.String("saas-rds-security-group"), &ec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: aws.Bool(true),
	})
	rdsSecurityGroup.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("172.31.0.0/16")), ec2.Port_Tcp(aws.Float64(3306)), aws.String("allow from vpc"))

	// 创建RDS参数组
	paramGrp := rds.NewParameterGroup(stack, aws.String("MyAurora"), &rds.ParameterGroupProps{
		Engine: rds.DatabaseClusterEngine_AuroraMysql(&rds.AuroraMysqlClusterEngineProps{
			Version: rds.AuroraMysqlEngineVersion_VER_3_01_1(),
		}),
		Description: aws.String("Custom ParameterGroup"),
		Parameters:  &map[string]*string{
			//对应参数在这里修改
			// "event_scheduler":        aws.String("ON"),
			// "innodb_sync_array_size": aws.String("16"),
		},
	})

	// 创建RDS的数据库密码
	dbSecret := secretmgr.NewSecret(stack, aws.String("DBSecret"), &secretmgr.SecretProps{
		SecretName: aws.String(*stack.StackName() + "-Secret"),
		GenerateSecretString: &secretmgr.SecretStringGenerator{
			SecretStringTemplate: aws.String(fmt.Sprintf(`{"username":"%s"}`, "Passw0rd@")), //这里才是密码
			ExcludePunctuation:   aws.Bool(true),
			IncludeSpace:         aws.Bool(false),
			GenerateStringKey:    aws.String("password"),
		},
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})
	// 创建Aurora集群
	rdsCluster := rds.NewDatabaseCluster(stack, aws.String("saas-rds-cluster"), &rds.DatabaseClusterProps{
		Engine:         rds.DatabaseClusterEngine_AuroraMysql(&rds.AuroraMysqlClusterEngineProps{Version: rds.AuroraMysqlEngineVersion_VER_3_01_1()}),
		Vpc:            vpc,
		VpcSubnets:     &ec2.SubnetSelection{SubnetType: ec2.SubnetType_PRIVATE_ISOLATED},
		SecurityGroups: &[]ec2.ISecurityGroup{rdsSecurityGroup},
		ParameterGroup: paramGrp,
		RemovalPolicy:  cdk.RemovalPolicy_DESTROY,
		Port:           aws.Float64(3306),
		Credentials: rds.Credentials_FromPassword(
			aws.String("admin"),
			cdk.SecretValue_UnsafePlainText(dbSecret.SecretValueFromJson(aws.String("password")).UnsafeUnwrap()),
		),
		Writer: rds.ClusterInstance_Provisioned(aws.String("writer"), &rds.ProvisionedClusterInstanceProps{
			InstanceType: ec2.InstanceType_Of(ec2.InstanceClass_T3, ec2.InstanceSize_MEDIUM),
		}),
		Readers: &[]rds.IClusterInstance{
			rds.ClusterInstance_Provisioned(aws.String("reader"), &rds.ProvisionedClusterInstanceProps{
				InstanceType: ec2.InstanceType_Of(ec2.InstanceClass_T3, ec2.InstanceSize_MEDIUM),
			}),
		},
	})

	//输出
	cdk.NewCfnOutput(stack, aws.String("RDSEndpoint"), &awscdk.CfnOutputProps{
		Value: aws.String(*rdsCluster.ClusterReadEndpoint().Hostname()),
	})

	//msk 不是默认的L2 construct ,  需要单独使用go get 安装
	//go get github.com/aws/aws-cdk-go/awscdkmskalpha/v2
	// 创建MSK集群
	mskCluster := msk.NewCluster(stack, aws.String("Cluster"), &msk.ClusterProps{
		ClusterName:  aws.String("myCluster"),
		KafkaVersion: msk.KafkaVersion_V2_8_1(),
		Vpc:          vpc,
		VpcSubnets:   &ec2.SubnetSelection{SubnetType: ec2.SubnetType_PRIVATE_ISOLATED},
		//InstanceType: ec2.InstanceType_Of(ec2.InstanceClass_M5, ec2.InstanceSize_LARGE),
	})

	mskCluster.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("172.31.0.0/16")), ec2.Port_Tcp(aws.Float64(2181)), aws.String("msk connection"))
	mskCluster.Connections().AllowFrom(ec2.Peer_Ipv4(aws.String("172.31.0.0/16")), ec2.Port_Tcp(aws.Float64(9094)), aws.String("msk connection"))

	cdk.NewCfnOutput(stack, aws.String("MSKEndpoint"), &awscdk.CfnOutputProps{
		Value: aws.String(*mskCluster.BootstrapBrokersTls()),
	})
	cdk.NewCfnOutput(stack, aws.String("MSKZookeeper"), &awscdk.CfnOutputProps{
		Value: aws.String(*mskCluster.ZookeeperConnectionString()),
	})

	var subnetIds []*string
	subnets := vpc.PrivateSubnets()
	for _, subnet := range *subnets {
		subnetIds = append(subnetIds, subnet.SubnetId())
	}

	//创建ElastiCache 子网组
	redisSubnetGroup := elasticache.NewCfnSubnetGroup(
		stack,
		aws.String("RedisSubnetGroup"),
		&elasticache.CfnSubnetGroupProps{
			Description: aws.String("subnet group for redis"),
			SubnetIds:   &subnetIds,
		},
	)

	//集群模式需要使用NewCfnReplicationGroup
	//创建ElastiCache Redis 集群
	cfnCacheCluster := elasticache.NewCfnReplicationGroup(stack, aws.String("MyCfnCacheCluster"), &elasticache.CfnReplicationGroupProps{
		CacheNodeType: aws.String("cache.r6g.large"),
		Engine:        aws.String("redis"),
		//NumCacheClusters:            aws.Float64(1),
		NumNodeGroups:               aws.Float64(2),
		ClusterMode:                 aws.String("Enabled"),
		ReplicationGroupDescription: aws.String("redis-cluster-01"),

		// the properties below are optional
		AutoMinorVersionUpgrade: aws.Bool(false),

		//CacheParameterGroupName: aws.String("myredis-cluster"),

		CacheSubnetGroupName: redisSubnetGroup.Ref(),
		EngineVersion:        aws.String("7.0"),
		IpDiscovery:          aws.String("ipv4"),
		//LogDeliveryConfigurations: []interface{}{},
		NetworkType: aws.String("ipv4"),

		TransitEncryptionEnabled: aws.Bool(false),
	})

	cdk.NewCfnOutput(stack, aws.String("ElastiCacheEndpoint"), &awscdk.CfnOutputProps{
		Value: aws.String(fmt.Sprintf("%s:%s", *cfnCacheCluster.AttrConfigurationEndPointAddress(), *cfnCacheCluster.AttrConfigurationEndPointPort())),
	})

	// 创建单节点redis
	// cfnCacheClusterSingle := elasticache.NewCfnCacheCluster(stack, aws.String("MyCfnCacheCluster"), &elasticache.CfnCacheClusterProps{
	// 	CacheNodeType: aws.String("cache.r6g.large"),
	// 	Engine:        aws.String("redis"),
	// 	NumCacheNodes: aws.Float64(1),

	// 	// the properties below are optional
	// 	AutoMinorVersionUpgrade: aws.Bool(false),
	// 	AzMode:                  aws.String("single-az"),
	// 	//CacheParameterGroupName: aws.String("myredis-cluster"),

	// 	CacheSubnetGroupName: redisSubnetGroup.Ref(),
	// 	ClusterName:          aws.String("redis-cluster-01"),
	// 	EngineVersion:        aws.String("7.0"),
	// 	IpDiscovery:          aws.String("ipv4"),
	// 	//LogDeliveryConfigurations: []interface{}{},
	// 	NetworkType:               aws.String("ipv4"),
	// 	PreferredAvailabilityZone: aws.String("us-east-1a"),

	// 	TransitEncryptionEnabled: aws.Bool(false),
	// 	VpcSecurityGroupIds: &[]*string{
	// 		ec2SecurityGroup.SecurityGroupId(),
	// 	},
	// })

	// cdk.NewCfnOutput(stack, aws.String("ElastiCacheSingleEndpoint"), &awscdk.CfnOutputProps{
	// 	Value: aws.String(*cfnCacheClusterSingle.ClusterName()),
	// })

	//创建EMR集群
	emrCluster := emr.NewCfnCluster(stack, aws.String("EMRCluster"), &emr.CfnClusterProps{
		Name:         aws.String("MyEMRCluster"),
		ReleaseLabel: aws.String("emr-6.10.0"),
		Applications: &[]*emr.CfnCluster_ApplicationProperty{
			&emr.CfnCluster_ApplicationProperty{
				Name: aws.String("Spark"),
			},
			&emr.CfnCluster_ApplicationProperty{
				Name: aws.String("Flink"),
			},
		},
		Instances: &emr.CfnCluster_JobFlowInstancesConfigProperty{
			TerminationProtected: aws.Bool(false),
			MasterInstanceGroup: &emr.CfnCluster_InstanceGroupConfigProperty{
				InstanceCount: aws.Float64(1),
				InstanceType:  aws.String("c5.xlarge"),
			},
			CoreInstanceGroup: &emr.CfnCluster_InstanceGroupConfigProperty{
				InstanceCount: aws.Float64(1),
				InstanceType:  aws.String("c5.xlarge"),
			},
			Ec2SubnetId: subnetIds[0],
			//Ec2SubnetIds: &subnetIds,
		},
		JobFlowRole: aws.String("EMR_EC2_DefaultRole"),
		ServiceRole: aws.String("EMR_DefaultRole"),
	})

	cdk.NewCfnOutput(stack, aws.String("EMREndpoint"), &awscdk.CfnOutputProps{
		Value: aws.String(*emrCluster.AttrMasterPublicDns()),
	})

	//创建EC2 Role
	ssmPolicy := iam.ManagedPolicy_FromAwsManagedPolicyName(aws.String("AmazonSSMManagedInstanceCore"))
	instanceRole := iam.NewRole(stack, aws.String("webinstancerole"),
		&iam.RoleProps{
			AssumedBy:       iam.NewServicePrincipal(aws.String("ec2.amazonaws.com"), nil),
			Description:     aws.String("Instance Role"),
			ManagedPolicies: &[]iam.IManagedPolicy{ssmPolicy},
		},
	)

	//创建EBS卷
	volume := ec2.BlockDeviceVolume_Ebs(aws.Float64(60), &ec2.EbsDeviceOptions{
		VolumeType: ec2.EbsDeviceVolumeType_GP3,
	})
	rootVolume := &ec2.BlockDevice{
		DeviceName: aws.String("/dev/xvda"),
		Volume:     volume,
	}

	data, err := ioutil.ReadFile("userdata/userdata.sh")
	if err != nil {
		panic("File reading error")
	}
	userdataContent := string(data)
	userdata := ec2.UserData_Custom(&userdataContent)

	//设置EC2使用的AMI ID
	amazonLinuxImage := ec2.NewGenericLinuxImage(
		&map[string]*string{
			"us-east-1": aws.String("ami-0277155c3f0ab2930"),
		},
		&ec2.GenericLinuxImageProps{},
	)

	//创建EC2实例
	webserver := ec2.NewInstance(stack, aws.String("webserver"),
		&ec2.InstanceProps{
			InstanceType:  ec2.InstanceType_Of(ec2.InstanceClass_MEMORY5_AMD, ec2.InstanceSize_LARGE),
			MachineImage:  amazonLinuxImage,
			BlockDevices:  &[]*ec2.BlockDevice{rootVolume},
			Vpc:           vpc,
			InstanceName:  aws.String("monolith"),
			Role:          instanceRole,
			SecurityGroup: webServerSecurityGroup,
			UserData:      userdata,
			VpcSubnets: &ec2.SubnetSelection{
				SubnetType: ec2.SubnetType_PUBLIC,
			},
			KeyName: aws.String("wsu-us-east-1"),
		})

	cdk.NewCfnOutput(stack, aws.String("WebServerPublicDNS"), &awscdk.CfnOutputProps{
		Value: aws.String(*webserver.InstancePublicDnsName()),
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
