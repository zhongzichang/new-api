package setting

var (
	UsdcEnabled         bool
	UsdcEthRpcUrl       string
	UsdcBscRpcUrl       string
	UsdcBaseRpcUrl      string
	UsdcEthContract     string = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	UsdcBscContract     string = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
	UsdcBaseContract    string = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	UsdcEthReceiver     string
	UsdcBscReceiver     string
	UsdcBaseReceiver    string
	UsdcEthDecimals     int     = 6
	UsdcBscDecimals     int     = 6
	UsdcBaseDecimals    int     = 6
	UsdcMinTopUp        int     = 1
	UsdcUnitPrice       float64 = 1.0
	UsdcEthConfirmations        = 3
	UsdcBscConfirmations        = 6
	UsdcBaseConfirmations       = 6
	UsdcTimeoutMinutes  int     = 60
)
