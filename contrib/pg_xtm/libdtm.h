#ifndef LIBDTM_H
#define LIBDTM_H

#include "postgres.h"
#include "utils/snapmgr.h"
#include "access/clog.h"

#define INVALID_XID 0

typedef int NodeId;
typedef unsigned long long xid_t;

typedef struct DTMConnData *DTMConn;

// Connects to the specified DTM.
DTMConn DtmConnect(char *host, int port);

// Disconnects from the DTM. Do not use the 'dtm' pointer after this call, or
// bad things will happen.
void DtmDisconnect(DTMConn dtm);

typedef struct {
    TransactionId* xids;
    NodeId* nodes;
    int nNodes;
} GlobalTransactionId;

// Creates an entry for a new global transaction. Returns 'true' on success, or
// 'false' otherwise.
bool DtmGlobalStartTransaction(DTMConn dtm, GlobalTransactionId* gtid);

// Asks DTM for a fresh snapshot. Returns 'true' on success, or 'false'
// otherwise.
bool DtmGlobalGetSnapshot(DTMConn dtm, NodeId nodeid, TransactionId xid, Snapshot snapshot);

// Commits transaction only once all participants have called this function,
// does not change CLOG otherwise. Returns 'true' on success, 'false' if
// something failed on the daemon side.
bool DtmGlobalSetTransStatus(DTMConn dtm, NodeId nodeid, TransactionId xid, XidStatus status);

// Gets the status of the transaction identified by 'xid'. Returns the status
// on success, or -1 otherwise.
XidStatus DtmGlobalGetTransStatus(DTMConn dtm, NodeId nodeid, TransactionId xid);

#endif
