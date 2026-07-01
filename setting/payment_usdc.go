package setting

var (
	UsdcEnabled         bool
	UsdcEthRpcUrl       string
	UsdcBscRpcUrl       string
	UsdcEthContract     string = "0xA0b86a33E6441E6C7D3D4B4f6b1EeC8d4F0B7b2a"
	UsdcBscContract     string = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
	UsdcEthReceiver     string
	UsdcBscReceiver     string
	UsdcEthDecimals     int    = 6
	UsdcBscDecimals     int    = 18
	UsdcMinTopUp        int    = 1
	UsdcUnitPrice       float64 = 1.0
	UsdcEthConfirmations       = 3
	UsdcBscConfirmations       = 6
	UsdcTimeoutMinutes  int    = 60
)
