package investment

import (
	"fmt"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/google/uuid"
)

/*
FundingTransaction returns a balanced ledger transaction: principal debited to the investor's

investment account and credited to aggregate investor capital.
*/
func FundingTransaction(investorLedgerAccountID, investorCapitalAccountID uuid.UUID, principal int64, investmentID uuid.UUID) domain.Transaction {
	return domain.Transaction{
		Reference: fmt.Sprintf("Investment_created_%s", investmentID),
		Entries: []domain.Entry{
			{
				AccountId: investorLedgerAccountID,
				Amount:    principal,
				Type:      domain.Debit,
			},
			{
				AccountId: investorCapitalAccountID,
				Amount:    principal,
				Type:      domain.Credit,
			},
		},
	}
}
