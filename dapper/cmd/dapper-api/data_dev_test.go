// +build devtest

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/segmentio/go-athena"
	"log"
	"strings"
	"testing"
	"time"
)

const athenaDataSpan = `SELECT timestamp, value FROM ingest WHERE domain='%s' AND key='%s' AND field='%s' AND timestamp >= '%s' AND timestamp <= '%s' %s ORDER BY timestamp DESC;`
const athenaFields = `SELECT distinct(field) FROM ingest WHERE domain='%s' AND key='%s';`
const athenaDataLatest = `SELECT timestamp, value FROM ingest WHERE domain='%s' AND key='%s' AND field='%s' ORDER BY timestamp DESC LIMIT %d;`
const athenaHasNewRec = `SELECT record_key FROM ingest WHERE domain='%s' AND timestamp > '%s' %s ORDER BY timestamp DESC`

func makePartitionClause(start, end string) string {
	st, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return ""
	}

	et, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return ""
	}

	sy := st.Year()
	ey := et.Year()

	sql := ""
	for y := sy; y <= ey; y++ {
		var startM int
		var endM int
		if sql == "" {
			// first year, starts from start month
			startM = int(st.Month())
		} else {
			// next year
			sql += " OR "
			startM = 1
		}

		sql += fmt.Sprintf("(year='%d' AND (", y)

		if y == ey {
			endM = int(et.Month())
		} else {
			endM = 12
		}

		for m := startM; m <= endM; m++ {
			sql += fmt.Sprintf(" month='%02d' OR", m)
		}

		sql = strings.TrimSuffix(sql, " OR") // remove extra "OR"
		sql += "))"                          // end of months and year
	}

	sql = " AND (" + sql + ") "

	return sql
}

func TestMakePartitionClause(t *testing.T) {
	s := makePartitionClause("2020-12-10T23:45:40Z", "2020-12-11T23:45:40Z")
	es := ` AND ((year='2020' AND ( month='12'))) `
	if s != es {
		t.Errorf("generated partition clause \"%s\"doesn't match as expected.", s)
	}

	s = makePartitionClause("2019-12-10T23:45:40Z", "2020-12-10T23:45:40Z")
	es = ` AND ((year='2019' AND ( month='12')) OR (year='2020' AND ( month='01' OR month='02' OR month='03' OR month='04' OR month='05' OR month='06' OR month='07' OR month='08' OR month='09' OR month='10' OR month='11' OR month='12'))) `

	if s != es {
		t.Errorf("generated partition clause \"%s\"doesn't match as expected.", s)
	}
}

func TestAthenaQuery(t *testing.T) {
	db, err := sql.Open("athena", "db=test-dapper&output_location=s3://test-howard/athena")
	if err != nil {
		t.Fatal(err)
	}

	sql := fmt.Sprintf(athenaFields, "fdmp", "wansw04-avalonlab")
	log.Println(sql)
	rows, err := db.Query(sql)
	if err != nil {
		t.Fatal(err)
	}

	for rows.Next() {
		var field string
		err = rows.Scan(&field)
		if err != nil {
			t.Error(err)
		}
		fmt.Println(field)
	}

	sql = fmt.Sprintf(athenaDataLatest, "fdmp", "wansw04-avalonlab", "temperature", 10)
	log.Println(sql)
	rows, err = db.Query(sql)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var timestamp, value string
		err = rows.Scan(&timestamp, &value)
		if err != nil {
			t.Error(err)
		}
		fmt.Println(timestamp, value)
	}

	st := "2020-11-10T23:45:40Z"
	et := "2020-12-05T23:45:40Z"
	sql = fmt.Sprintf(athenaDataSpan, "fdmp", "wansw04-avalonlab", "temperature", st, et, makePartitionClause(st, et))
	log.Println(sql)
	rows, err = db.Query(sql)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var timestamp, value string
		err = rows.Scan(&timestamp, &value)
		if err != nil {
			t.Error(err)
		}
		fmt.Println(timestamp, value)
	}
}
