package setting

var (
	UsdtEnabled          bool
	UsdtEthRpcUrl        string
	UsdtBscRpcUrl        string
	UsdtBaseRpcUrl       string
	UsdtPolygonRpcUrl    string
	UsdtTronRpcUrl       string
	UsdtEthContract      string = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	UsdtBscContract      string = "0x55d398326f99059fF775485246999027B3197955"
	UsdtBaseContract     string = "0xfde4C96c8593536E31F229EA8f37b2ADa2699bb2"
	UsdtPolygonContract  string = "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"
	UsdtTronContract     string = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	UsdtEthReceiver      string
	UsdtBscReceiver      string
	UsdtBaseReceiver     string
	UsdtPolygonReceiver  string
	UsdtTronReceiver     string
	UsdtEthDecimals      int     = 6
	UsdtBscDecimals      int     = 18
	UsdtBaseDecimals     int     = 6
	UsdtPolygonDecimals  int     = 6
	UsdtTronDecimals     int     = 6
	UsdtMinTopUp         int     = 1
	UsdtUnitPrice        float64 = 1.0
	UsdtEthConfirmations         = 3
	UsdtBscConfirmations         = 6
	UsdtBaseConfirmations        = 6
	UsdtPolygonConfirmations     = 12
	UsdtTronConfirmations        = 19
	UsdtTimeoutMinutes   int     = 60
)
