package controller

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	troncommon "github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

const usdtTransferABI = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`

var usdtABI abi.ABI

func init() {
	parsedABI, err := abi.JSON(strings.NewReader(usdtTransferABI))
	if err == nil {
		usdtABI = parsedABI
	}
}

type UsdtPayRequest struct {
	Amount int64  `json:"amount"`
	Chain  string `json:"chain"`
}

type UsdtVerifyRequest struct {
	TradeNo string `json:"trade_no"`
	TxHash  string `json:"tx_hash"`
}

type UsdtTxHashRequest struct {
	TradeNo string `json:"trade_no"`
	TxHash  string `json:"tx_hash"`
}

func UpdateUsdtTxHash(c *gin.Context) {
	var req UsdtTxHashRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" || req.TxHash == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	topUp := model.GetTopUpByTradeNo(req.TradeNo)
	if topUp == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值订单不存在"})
		return
	}

	userId := c.GetInt("id")
	if topUp.UserId != userId {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "无权操作该订单"})
		return
	}

	if topUp.Status != common.TopUpStatusPending {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值订单状态错误"})
		return
	}

	if topUp.PaymentProvider != model.PaymentProviderUsdtEth && topUp.PaymentProvider != model.PaymentProviderUsdtBsc && topUp.PaymentProvider != model.PaymentProviderUsdtBase && topUp.PaymentProvider != model.PaymentProviderUsdtPolygon && topUp.PaymentProvider != model.PaymentProviderUsdtTron {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "非 USDT 充值订单"})
		return
	}

	txHash := strings.TrimSpace(req.TxHash)
	if !strings.HasPrefix(txHash, "0x") {
		txHash = "0x" + txHash
	}

	topUp.TxHash = txHash
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("USDT 更新交易哈希失败 trade_no=%s tx=%s error=%q", req.TradeNo, txHash, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "更新交易哈希失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": "更新成功"})
}
func isUsdtTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.UsdtEnabled {
		return false
	}
	if strings.TrimSpace(setting.UsdtEthReceiver) == "" && strings.TrimSpace(setting.UsdtBscReceiver) == "" && strings.TrimSpace(setting.UsdtBaseReceiver) == "" && strings.TrimSpace(setting.UsdtPolygonReceiver) == "" && strings.TrimSpace(setting.UsdtTronReceiver) == "" {
		return false
	}
	return true
}

func getUsdtChainConfig(chain string) (rpcURL, contractAddress, receiver string, decimals int, confirmations int) {
	switch strings.ToLower(chain) {
	case "bsc":
		return setting.UsdtBscRpcUrl,
			setting.UsdtBscContract,
			setting.UsdtBscReceiver,
			setting.UsdtBscDecimals,
			setting.UsdtBscConfirmations
	case "base":
		return setting.UsdtBaseRpcUrl,
			setting.UsdtBaseContract,
			setting.UsdtBaseReceiver,
			setting.UsdtBaseDecimals,
			setting.UsdtBaseConfirmations
	case "polygon":
		return setting.UsdtPolygonRpcUrl,
			setting.UsdtPolygonContract,
			setting.UsdtPolygonReceiver,
			setting.UsdtPolygonDecimals,
			setting.UsdtPolygonConfirmations
	case "tron":
		return setting.UsdtTronRpcUrl,
			setting.UsdtTronContract,
			setting.UsdtTronReceiver,
			setting.UsdtTronDecimals,
			setting.UsdtTronConfirmations
	default:
		return setting.UsdtEthRpcUrl,
			setting.UsdtEthContract,
			setting.UsdtEthReceiver,
			setting.UsdtEthDecimals,
			setting.UsdtEthConfirmations
	}
}

func normalizeUsdtAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		return decimal.NewFromInt(amount).
			Div(decimal.NewFromFloat(common.QuotaPerUnit)).
			IntPart()
	}
	return amount
}

func getUsdtPayMoney(amount int64, group string) float64 {
	originalAmount := amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = int64(math.Round(float64(amount) / common.QuotaPerUnit))
		if amount < 1 {
			amount = 1
		}
	}

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok && ds > 0 {
		discount = ds
	}

	return float64(amount) * setting.UsdtUnitPrice * topupGroupRatio * discount
}

func RequestUsdtAmount(c *gin.Context) {
	var req UsdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < int64(setting.UsdtMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", setting.UsdtMinTopUp)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getUsdtPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": strconv.FormatFloat(payMoney, 'f', 6, 64)})
}

func RequestUsdtPay(c *gin.Context) {
	if !isUsdtTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "USDT 充值未启用"})
		return
	}

	var req UsdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	chain := strings.ToLower(req.Chain)
	if chain != "eth" && chain != "bsc" && chain != "base" && chain != "polygon" && chain != "tron" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的链"})
		return
	}

	if req.Amount < int64(setting.UsdtMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", setting.UsdtMinTopUp)})
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "用户不存在"})
		return
	}

	_, _, receiver, decimals, requiredConfirmations := getUsdtChainConfig(chain)
	if strings.TrimSpace(receiver) == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "该链收款地址未配置"})
		return
	}

	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getUsdtPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	paymentMethod := model.PaymentMethodUsdtEth
	paymentProvider := model.PaymentProviderUsdtEth
	if chain == "bsc" {
		paymentMethod = model.PaymentMethodUsdtBsc
		paymentProvider = model.PaymentProviderUsdtBsc
	} else if chain == "base" {
		paymentMethod = model.PaymentMethodUsdtBase
		paymentProvider = model.PaymentProviderUsdtBase
	} else if chain == "polygon" {
		paymentMethod = model.PaymentMethodUsdtPolygon
		paymentProvider = model.PaymentProviderUsdtPolygon
	} else if chain == "tron" {
		paymentMethod = model.PaymentMethodUsdtTron
		paymentProvider = model.PaymentProviderUsdtTron
	}

	tradeNo := fmt.Sprintf("USDT-%s-%d-%d-%s", strings.ToUpper(chain), id, time.Now().UnixMilli(), randstr.String(6))
	topUp := &model.TopUp{
		UserId:                id,
		Amount:                normalizeUsdtAmount(req.Amount),
		Money:                 payMoney,
		TradeNo:               tradeNo,
		PaymentMethod:         paymentMethod,
		PaymentProvider:       paymentProvider,
		CreateTime:            time.Now().Unix(),
		Status:                common.TopUpStatusPending,
		RequiredConfirmations: requiredConfirmations,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("USDT 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	expiresAt := time.Now().Add(time.Duration(setting.UsdtTimeoutMinutes) * time.Minute).Unix()
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("USDT 充值订单创建成功 user_id=%d trade_no=%s chain=%s amount=%d money=%.6f", id, tradeNo, chain, req.Amount, payMoney))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "success",
		"data": gin.H{
			"trade_no":       tradeNo,
			"chain":          chain,
			"receiver":       receiver,
			"amount":         payMoney,
			"token_decimals": decimals,
			"expires_at":     expiresAt,
		},
	})
}

func CancelUsdtTopUp(c *gin.Context) {
	var req struct {
		TradeNo string `json:"trade_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	userId := c.GetInt("id")
	topUp := model.GetTopUpByTradeNo(req.TradeNo)
	if topUp == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值订单不存在"})
		return
	}
	if topUp.UserId != userId {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "无权操作该订单"})
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单状态不允许取消"})
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderUsdtEth && topUp.PaymentProvider != model.PaymentProviderUsdtBsc && topUp.PaymentProvider != model.PaymentProviderUsdtBase && topUp.PaymentProvider != model.PaymentProviderUsdtPolygon && topUp.PaymentProvider != model.PaymentProviderUsdtTron {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单类型不支持取消"})
		return
	}

	topUp.Status = common.TopUpStatusCancelled
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("USDT 取消订单失败 user_id=%d trade_no=%s error=%q", userId, req.TradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "取消订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("USDT 取消订单成功 user_id=%d trade_no=%s", userId, req.TradeNo))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": "订单已取消"})
}

func VerifyUsdtTransaction(c *gin.Context) {
	var req UsdtVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" || req.TxHash == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	topUp := model.GetTopUpByTradeNo(req.TradeNo)
	if topUp == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值订单不存在"})
		return
	}

	if topUp.Status != common.TopUpStatusPending {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值订单状态错误"})
		return
	}

	chain := "eth"
	if topUp.PaymentProvider == model.PaymentProviderUsdtBsc {
		chain = "bsc"
	} else if topUp.PaymentProvider == model.PaymentProviderUsdtBase {
		chain = "base"
	} else if topUp.PaymentProvider == model.PaymentProviderUsdtPolygon {
		chain = "polygon"
	} else if topUp.PaymentProvider == model.PaymentProviderUsdtTron {
		chain = "tron"
	}

	rpcURL, contractAddress, receiver, decimals, requiredConfirmations := getUsdtChainConfig(chain)
	if strings.TrimSpace(rpcURL) == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "该链 RPC 未配置"})
		return
	}

	txHash := strings.TrimSpace(req.TxHash)

	var amount float64
	var confirmations int
	var err error
	if chain == "tron" {
		amount, confirmations, err = verifyUsdtTransferOnTron(rpcURL, contractAddress, receiver, txHash, decimals)
	} else {
		if !strings.HasPrefix(txHash, "0x") {
			txHash = "0x" + txHash
		}
		amount, confirmations, err = verifyUsdtTransferOnChain(context.Background(), rpcURL, contractAddress, receiver, txHash, decimals)
	}
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("USDT 链上验证失败 trade_no=%s tx=%s error=%q", req.TradeNo, txHash, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	if topUp.TxHash == "" {
		topUp.TxHash = txHash
	}
	if topUp.Confirmations != confirmations {
		topUp.Confirmations = confirmations
		if err := topUp.Update(); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("USDT 更新确认数失败 trade_no=%s error=%q", req.TradeNo, err.Error()))
		}
	}

	if confirmations < requiredConfirmations {
		c.JSON(http.StatusOK, gin.H{
			"message": "error",
			"data":  fmt.Sprintf("等待更多区块确认，当前 %d/%d", confirmations, requiredConfirmations),
		})
		return
	}

	expectedAmount := decimal.NewFromFloat(topUp.Money)
	actualAmount := decimal.NewFromFloat(amount)
	tolerance := expectedAmount.Mul(decimal.NewFromFloat(0.01))
	minAccepted := expectedAmount.Sub(tolerance)
	maxAccepted := expectedAmount.Add(tolerance)

	if actualAmount.LessThan(minAccepted) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("到账金额不足，预期 %.6f，实际 %.6f", topUp.Money, amount)})
		return
	}
	if actualAmount.GreaterThan(maxAccepted) {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("USDT 到账金额超过预期 trade_no=%s expected=%.6f actual=%.6f", req.TradeNo, topUp.Money, amount))
	}

	LockOrder(req.TradeNo)
	defer UnlockOrder(req.TradeNo)

	if err := model.RechargeUsdt(req.TradeNo, txHash, c.ClientIP()); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("USDT 充值失败 trade_no=%s tx=%s error=%q", req.TradeNo, txHash, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("USDT 充值成功 trade_no=%s tx=%s amount=%.6f confirmations=%d", req.TradeNo, txHash, amount, confirmations))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": "充值成功"})
}

func verifyUsdtTransferOnTron(apiURL, contractAddress, receiver, txHash string, decimals int) (float64, int, error) {
	if strings.TrimSpace(apiURL) == "" {
		apiURL = "https://api.trongrid.io"
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return 0, 0, errors.New("交易哈希为空")
	}

	tronApiKey := strings.TrimSpace(setting.TronGridApiKey)

	// 1. Verify transaction info
	infoURL := fmt.Sprintf("%s/wallet/gettransactioninfobyid", strings.TrimRight(apiURL, "/"))
	infoBody := fmt.Sprintf(`{"value":"%s"}`, txHash)
	infoReq, err := http.NewRequest(http.MethodPost, infoURL, strings.NewReader(infoBody))
	if err != nil {
		return 0, 0, fmt.Errorf("构建交易信息请求失败: %w", err)
	}
	infoReq.Header.Set("Content-Type", "application/json")
	infoReq.Header.Set("Accept", "application/json")
	if tronApiKey != "" {
		infoReq.Header.Set("TRON-PRO-API-KEY", tronApiKey)
	}
	infoResp, err := http.DefaultClient.Do(infoReq)
	if err != nil {
		return 0, 0, fmt.Errorf("查询交易信息失败: %w", err)
	}
	defer infoResp.Body.Close()
	infoRespBody, _ := io.ReadAll(infoResp.Body)
	if infoResp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("查询交易信息失败: status=%d", infoResp.StatusCode)
	}
	var txInfo struct {
		ID       string `json:"id"`
		BlockNumber int64 `json:"blockNumber"`
		Receipt  struct {
			Result string `json:"result"`
		} `json:"receipt"`
		Log []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"log"`
	}
	if err := common.UnmarshalJsonStr(string(infoRespBody), &txInfo); err != nil {
		return 0, 0, fmt.Errorf("解析交易信息失败: %w", err)
	}
	if txInfo.ID == "" {
		return 0, 0, errors.New("交易不存在")
	}
	if txInfo.Receipt.Result != "SUCCESS" {
		return 0, 0, errors.New("交易执行失败")
	}

	// Decode receiver and contract address to hex form (without 0x41 prefix)
	receiverBytes, err := troncommon.DecodeCheck(receiver)
	if err != nil {
		return 0, 0, fmt.Errorf("收款地址解析失败: %w", err)
	}
	receiverHex := hex.EncodeToString(receiverBytes[1:])
	contractBytes, err := troncommon.DecodeCheck(contractAddress)
	if err != nil {
		return 0, 0, fmt.Errorf("合约地址解析失败: %w", err)
	}
	contractHex := hex.EncodeToString(contractBytes[1:])

	transferSig := hex.EncodeToString(usdtABI.Events["Transfer"].ID.Bytes())
	var matchedAmount *big.Int
	for _, l := range txInfo.Log {
		if !strings.EqualFold(l.Address, contractHex) {
			continue
		}
		if len(l.Topics) < 3 {
			continue
		}
		if !strings.EqualFold(strings.TrimPrefix(l.Topics[0], "0x"), transferSig) {
			continue
		}
		toTopic := strings.TrimPrefix(l.Topics[2], "0x")
		if len(toTopic) < 40 {
			continue
		}
		toAddr := toTopic[len(toTopic)-40:]
		if !strings.EqualFold(toAddr, receiverHex) {
			continue
		}
		data := strings.TrimPrefix(l.Data, "0x")
		value, ok := new(big.Int).SetString(data, 16)
		if !ok {
			continue
		}
		matchedAmount = value
		break
	}
	if matchedAmount == nil {
		return 0, 0, errors.New("未找到向指定收款地址的 USDT 转账")
	}

	// 2. Get current block for confirmations
	nowBlockURL := fmt.Sprintf("%s/wallet/getnowblock", strings.TrimRight(apiURL, "/"))
	nowReq, err := http.NewRequest(http.MethodPost, nowBlockURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("构建当前区块请求失败: %w", err)
	}
	nowReq.Header.Set("Accept", "application/json")
	if tronApiKey != "" {
		nowReq.Header.Set("TRON-PRO-API-KEY", tronApiKey)
	}
	nowResp, err := http.DefaultClient.Do(nowReq)
	if err != nil {
		return 0, 0, fmt.Errorf("查询当前区块失败: %w", err)
	}
	defer nowResp.Body.Close()
	nowRespBody, _ := io.ReadAll(nowResp.Body)
	if nowResp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("查询当前区块失败: status=%d", nowResp.StatusCode)
	}
	var nowBlock struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := common.UnmarshalJsonStr(string(nowRespBody), &nowBlock); err != nil {
		return 0, 0, fmt.Errorf("解析当前区块失败: %w", err)
	}
	confirmations := int(nowBlock.BlockHeader.RawData.Number - txInfo.BlockNumber)
	if confirmations < 0 {
		confirmations = 0
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountFloat, _ := new(big.Rat).SetFrac(matchedAmount, divisor).Float64()
	return amountFloat, confirmations, nil
}

func verifyUsdtTransferOnChain(ctx context.Context, rpcURL, contractAddress, receiver, txHash string, decimals int) (float64, int, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return 0, 0, fmt.Errorf("连接节点失败: %w", err)
	}
	defer client.Close()

	txHashBytes := ethcommon.HexToHash(txHash)
	receipt, err := client.TransactionReceipt(ctx, txHashBytes)
	if err != nil {
		return 0, 0, fmt.Errorf("获取交易回执失败: %w", err)
	}

	if receipt.Status != 1 {
		return 0, 0, errors.New("交易执行失败")
	}

	if len(receipt.Logs) == 0 {
		return 0, 0, errors.New("交易中没有事件日志")
	}

	contractAddr := ethcommon.HexToAddress(contractAddress)
	receiverAddr := ethcommon.HexToAddress(receiver)

	var matchedAmount *big.Int
	transferSig := usdtABI.Events["Transfer"].ID

	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 || log.Topics[0] != transferSig {
			continue
		}
		if !strings.EqualFold(log.Address.Hex(), contractAddr.Hex()) {
			continue
		}
		if len(log.Topics) < 3 {
			continue
		}

		toAddr := ethcommon.BytesToAddress(log.Topics[2].Bytes())
		if !strings.EqualFold(toAddr.Hex(), receiverAddr.Hex()) {
			continue
		}

		if len(log.Data) < 32 {
			continue
		}

		value := new(big.Int).SetBytes(log.Data[:32])
		matchedAmount = value
		break
	}

	if matchedAmount == nil {
		return 0, 0, errors.New("未找到向指定收款地址的 USDT 转账")
	}

	currentBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取当前区块失败: %w", err)
	}

	confirmations := int(currentBlock - receipt.BlockNumber.Uint64())
	if currentBlock < receipt.BlockNumber.Uint64() {
		confirmations = 0
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	amountFloat, _ := new(big.Rat).SetFrac(matchedAmount, divisor).Float64()

	return amountFloat, confirmations, nil
}
