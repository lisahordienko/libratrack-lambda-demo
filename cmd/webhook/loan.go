package main

import (
	"math"
	"regexp"
	"strings"
)

type LoanFineRequest struct {
	LoanID      string  `json:"loan_id"`
	DaysOverdue int     `json:"days_overdue"`
	DailyRate   float64 `json:"daily_rate"`
	Currency    string  `json:"currency"`
}

type LoanFine struct {
	LoanID      string  `json:"loan_id"`
	DaysOverdue int     `json:"days_overdue"`
	DailyRate   float64 `json:"daily_rate"`
	Currency    string  `json:"currency"`
	FineAmount  float64 `json:"fine_amount"`
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func calculateFine(in LoanFineRequest) (LoanFine, []string) {
	loanID := strings.TrimSpace(in.LoanID)
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	dailyRateCents := int64(math.Round(in.DailyRate * 100))
	var problems []string

	if loanID == "" {
		problems = append(problems, "loan_id is required")
	}
	if in.DaysOverdue < 1 || in.DaysOverdue > 365 {
		problems = append(problems, "days_overdue must be between 1 and 365")
	}
	if math.IsNaN(in.DailyRate) || math.IsInf(in.DailyRate, 0) || dailyRateCents < 1 || dailyRateCents > 10000 {
		problems = append(problems, "daily_rate must be between 0.01 and 100.00")
	}
	if !currencyCode.MatchString(currency) {
		problems = append(problems, "currency must be a three-letter code")
	}
	if len(problems) > 0 {
		return LoanFine{}, problems
	}

	return LoanFine{
		LoanID:      loanID,
		DaysOverdue: in.DaysOverdue,
		DailyRate:   float64(dailyRateCents) / 100,
		Currency:    currency,
		FineAmount:  float64(dailyRateCents*int64(in.DaysOverdue)) / 100,
	}, nil
}