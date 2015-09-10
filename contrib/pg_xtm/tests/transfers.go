package main

import (
    "fmt"
    "sync"
    "math/rand"
    "github.com/jackc/pgx"
)

const (
    TRANSFER_CONNECTIONS = 4
    INIT_AMOUNT = 10000
    N_ITERATIONS = 10000
    N_ACCOUNTS = 1//100000
)


var cfg1 = pgx.ConnConfig{
        Host:     "127.0.0.1",
        Port:     5432,
        Database: "postgres",
    }

var cfg2 = pgx.ConnConfig{
        Host:     "127.0.0.1",
        Port:     5433,
        Database: "postgres",
    }

var running = false
var nodes []int32 = []int32{0,1}

func prepare_db() {
    var xids []int32 = make([]int32, 2)

    conn1, err := pgx.Connect(cfg1)
    checkErr(err)
    defer conn1.Close()

    conn2, err := pgx.Connect(cfg2)
    checkErr(err)
    defer conn2.Close()

    exec(conn1, "drop extension if exists pg_dtm")
    exec(conn1, "create extension pg_dtm")
    exec(conn1, "drop table if exists t")
    exec(conn1, "create table t(u int primary key, v int)")

    exec(conn2, "drop extension if exists pg_dtm")
    exec(conn2, "create extension pg_dtm")
    exec(conn2, "drop table if exists t")
    exec(conn2, "create table t(u int primary key, v int)")
    
    // strt transaction
    exec(conn1, "begin")
    exec(conn2, "begin")
    
    // obtain XIDs of paticipants
    xids[0] = execQuery(conn1, "select txid_current()")
    xids[1] = execQuery(conn2, "select txid_current()")
    
    // register global transaction in DTMD
    exec(conn1, "select dtm_global_transaction($1, $2)", nodes, xids)
    
    // first global statement 
    exec(conn1, "select dtm_get_snapshot()")
    exec(conn2, "select dtm_get_snapshot()")
    
    for i := 0; i < N_ACCOUNTS; i++ {
        exec(conn1, "insert into t values($1, $2)", i, INIT_AMOUNT)
        exec(conn2, "insert into t values($1, $2)", i, INIT_AMOUNT)
    }
    
    // second global statement 
    exec(conn1, "select dtm_get_snapshot()")
    exec(conn2, "select dtm_get_snapshot()")
    
    // commit work
    exec(conn1, "commit")
    exec(conn2, "commit")
    // at this moment transaction should be globally committed
}

func max(a, b int64) int64 {
    if a >= b {
        return a
    } 
    return b
}

func transfer(id int, wg *sync.WaitGroup) {
    var err error
    var xids []int32 = make([]int32, 2)

    conn1, err := pgx.Connect(cfg1)
    checkErr(err)
    defer conn1.Close()

    conn2, err := pgx.Connect(cfg2)
    checkErr(err)
    defer conn2.Close()

    for i := 0; i < N_ITERATIONS; i++ {
        //amount := 2*rand.Intn(2) - 1
        amount := 1
        account1 := rand.Intn(N_ACCOUNTS) 
        account2 := rand.Intn(N_ACCOUNTS)

        // strt transaction
        exec(conn1, "begin")
        exec(conn2, "begin")
        
        // obtain XIDs of paticipants
        xids[0] = execQuery(conn1, "select txid_current()")
        xids[1] = execQuery(conn2, "select txid_current()")
        
        // register global transaction in DTMD
        exec(conn1, "select dtm_global_transaction($1, $2)", nodes, xids)
        
        // first global statement 
        exec(conn1, "select dtm_get_snapshot()")
        exec(conn2, "select dtm_get_snapshot()")
        
        exec(conn1, "update t set v = v + $1 where u=$2", amount, account1)
        exec(conn2, "update t set v = v - $1 where u=$2", amount, account2)
        
        // second global statement 
        exec(conn1, "select dtm_get_snapshot()")
        exec(conn2, "select dtm_get_snapshot()")
        
        // commit work
        exec(conn1, "commit")
        exec(conn2, "commit")
        // at this moment transaction should be globally committed
    }

    fmt.Println("Test completed")
    wg.Done()
}

func total() int32 {
    var err error
    var sum1 int32
    var sum2 int32
    var xids []int32 = make([]int32, 2)

    conn1, err := pgx.Connect(cfg1)
    checkErr(err)
    defer conn1.Close()

    conn2, err := pgx.Connect(cfg2)
    checkErr(err)
    defer conn2.Close()

    for { 
        exec(conn1, "begin transaction")
        exec(conn2, "begin transaction")
 
        // obtain XIDs of paticipants
        xids[0] = execQuery(conn1, "select txid_current()")
        xids[1] = execQuery(conn2, "select txid_current()")
        
        // register global transaction in DTMD
        exec(conn1, "select dtm_global_transaction($1, $2)", nodes, xids)

        exec(conn1, "select dtm_get_snapshot()")
        exec(conn2, "select dtm_get_snapshot()")

        sum1 = execQuery(conn1, "select sum(v) from t")
        sum2 = execQuery(conn2, "select sum(v) from t")

        exec(conn1, "commit")
        exec(conn2, "commit")

        return sum1 + sum2
    }
}

func totalrep(wg *sync.WaitGroup) {
    var prevSum int32 = 0 
    for running {
        sum := total()
        if (sum != prevSum) {
            fmt.Println("Total = ", sum)
            prevSum = sum
        }
    }
    wg.Done()
}

func main() {
    var transferWg sync.WaitGroup
    var inspectWg sync.WaitGroup

    prepare_db()

    transferWg.Add(TRANSFER_CONNECTIONS)
    for i:=0; i<TRANSFER_CONNECTIONS; i++ {
        go transfer(i, &transferWg)
    }
    running = true
    inspectWg.Add(1)
    go totalrep(&inspectWg)

    transferWg.Wait()
    running = false
    inspectWg.Wait()
}

func exec(conn *pgx.Conn, stmt string, arguments ...interface{}) {
    var err error
    _, err = conn.Exec(stmt, arguments... )
    checkErr(err)
}

func execQuery(conn *pgx.Conn, stmt string, arguments ...interface{}) int32 {
    var err error
    var result int64
    err = conn.QueryRow(stmt, arguments...).Scan(&result)
    checkErr(err)
    return int32(result)
}

func checkErr(err error) {
    if err != nil {
        panic(err)
    }
}

