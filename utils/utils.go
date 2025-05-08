package utils

import (
	"database/sql"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/pentops/sqrlx.go/sqrlx"
	"github.com/robfig/cron/v3"
)

var (
	NamespaceTrigger = uuid.MustParse("C8343AEB-AEC1-430D-A6C0-6E7D2086D168")
)

var ReadOnlyTxOptions = &sqrlx.TxOptions{
	ReadOnly:  true,
	Retryable: true,
	Isolation: sql.LevelReadCommitted,
}

var MutableTxOptions = &sqrlx.TxOptions{
	ReadOnly:  false,
	Retryable: true,
	Isolation: sql.LevelReadCommitted,
}

func NewIdempotentId62(data []byte) string {
	id := uuid.NewMD5(NamespaceTrigger, data)
	return base62String(id[:])
}

func NewIdempotentId(data []byte) string {
	return uuid.NewMD5(NamespaceTrigger, data).String()
}

// from github.com/pentops/j5/lib/id62/id62.go
func base62String(id []byte) string {
	var i big.Int
	i.SetBytes(id)
	str := i.Text(62)
	if len(str) < 22 {
		str = fmt.Sprintf("%022s", str)
	} else if len(str) > 22 {
		panic("base62 value is too large")
	}
	return str
}

func ValidateCronString(c string) error {
	_, err := cron.ParseStandard(c)
	if err != nil {
		return fmt.Errorf("invalid cron string: %w", err)
	}
	return nil
}
