// Package shop prices orders.
//
// Extra lines shift every function down.
package shop

// Discount returns the price after applying a percentage discount.
func Discount(price, pct int) int {
	if pct > 100 {
		pct = 100
	}
	return price - price*pct/100
}

// Shipping returns the shipping fee for an order total.
func Shipping(total int) int {
	if total >= 50 {
		return 0
	}
	return 5
}

// Clamp limits v to [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Tax returns VAT on an amount.
func Tax(amount int) int {
	if amount <= 0 {
		return 0
	}
	return amount / 5
}
