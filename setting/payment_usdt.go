package setting

var (
	UsdtEnabled         bool
	UsdtEthRpcUrl       string
	UsdtBscRpcUrl       string
	UsdtEthContract     string = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	UsdtBscContract     string = "0x55d398326f99059fF775485246999027B3197955"
	UsdtEthReceiver     string
	UsdtBscReceiver     string
	UsdtEthDecimals     int    = 6
	UsdtBscDecimals     int    = 18
	UsdtMinTopUp        int    = 1
	UsdtUnitPrice       float64 = 1.0
	UsdtEthConfirmations       = 3
	UsdtBscConfirmations       = 6
	UsdtTimeoutMinutes  int    = 60
)
