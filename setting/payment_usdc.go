package setting

var (
	UsdcEnabled         bool
	UsdcEthRpcUrl       string
	UsdcBscRpcUrl       string
	UsdcEthContract     string = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	UsdcBscContract     string = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
	UsdcEthReceiver     string
	UsdcBscReceiver     string
	UsdcEthDecimals     int    = 6
	UsdcBscDecimals     int    = 6
	UsdcMinTopUp        int    = 1
	UsdcUnitPrice       float64 = 1.0
	UsdcEthConfirmations       = 3
	UsdcBscConfirmations       = 6
	UsdcTimeoutMinutes  int    = 60
)
