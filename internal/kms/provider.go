package kms

import okkms "gitlab.okg.com/okcoin-commons/ok-kms-go"

func init() {
	sdkInit = okkms.Init
	sdkGetSecretValue = okkms.GetSecretValue
}
