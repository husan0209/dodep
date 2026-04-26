package domain

// CryptoCurrency represents a supported cryptocurrency
type CryptoCurrency string

const (
	// Bitcoin
	CryptoBTC CryptoCurrency = "BTC"
	// Ethereum
	CryptoETH CryptoCurrency = "ETH"
	// Tether ERC-20
	CryptoUSDTETH CryptoCurrency = "USDTETH"
	// Tether TRC-20
	CryptoUSDTTRX CryptoCurrency = "USDTTRX"
	// USD Coin
	CryptoUSDC CryptoCurrency = "USDC"
	// Litecoin
	CryptoLTC CryptoCurrency = "LTC"
	// Bitcoin Cash
	CryptoBCH CryptoCurrency = "BCH"
)

// SupportedDepositCurrencies returns all supported deposit currencies
func SupportedDepositCurrencies() []CryptoCurrency {
	return []CryptoCurrency{
		CryptoBTC,
		CryptoETH,
		CryptoUSDTETH,
		CryptoUSDTTRX,
		CryptoUSDC,
		CryptoLTC,
		CryptoBCH,
	}
}

// SupportedWithdrawalCurrencies returns all supported withdrawal currencies
func SupportedWithdrawalCurrencies() []CryptoCurrency {
	return []CryptoCurrency{
		CryptoBTC,
		CryptoETH,
		CryptoUSDTETH,
		CryptoUSDTTRX,
		CryptoUSDC,
		CryptoLTC,
		CryptoBCH,
	}
}

// IsDepositSupported checks if currency is supported for deposits
func (c CryptoCurrency) IsDepositSupported() bool {
	for _, currency := range SupportedDepositCurrencies() {
		if c == currency {
			return true
		}
	}
	return false
}

// IsWithdrawalSupported checks if currency is supported for withdrawals
func (c CryptoCurrency) IsWithdrawalSupported() bool {
	for _, currency := range SupportedWithdrawalCurrencies() {
		if c == currency {
			return true
		}
	}
	return false
}

// String returns the string representation
func (c CryptoCurrency) String() string {
	return string(c)
}

// NOWPaymentsCurrency returns the currency code for NOWPayments API
func (c CryptoCurrency) NOWPaymentsCurrency() string {
	switch c {
	case CryptoUSDTETH:
		return "USDT"
	case CryptoUSDTTRX:
		return "USDTTRC20"
	default:
		return string(c)
	}
}

// Network returns the blockchain network for the currency
func (c CryptoCurrency) Network() string {
	switch c {
	case CryptoBTC:
		return "bitcoin"
	case CryptoETH, CryptoUSDTETH, CryptoUSDC:
		return "ethereum"
	case CryptoUSDTTRX:
		return "tron"
	case CryptoLTC:
		return "litecoin"
	case CryptoBCH:
		return "bitcoincash"
	default:
		return "unknown"
	}
}
