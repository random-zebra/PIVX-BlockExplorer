#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from bitcoinrpc.authproxy import AuthServiceProxy

try:
    import http.client as httplib
except ImportError:
    import httplib
import json
import os
import requests

isTestnet = True

# RPC CREDS
rpc_user = "rpc"
rpc_pass = "pivxrpc"
rpc_host = "127.0.0.1"
rpc_port = "18049" if isTestnet else "8049"
rpc_url = "http://%s:%s@%s" % (rpc_user, rpc_pass, rpc_host)
bb_url = "https://testnet" if isTestnet else "https://explorer"
bb_url += ".pivx.link"
# CHAIN PARAMS
nFirstZPivBlock = 201576 if isTestnet else 863787
nLastPOWBlock = 200 if isTestnet else 259200

# FILENAME
supply_data_file = os.path.join("/opt/coins/blockbook/plot_data", "supply_data.json")
network_data_file = os.path.join("/opt/coins/blockbook/plot_data", "network_data.json")

# --------------------------------------------------
class ApiClient:

    def __init__(self):
        self.url = bb_url

    def checkResponse(self, method, param=""):
        url = self.url + "/api/v2/%s" % method
        if param != "":
            url += "/%s" % param
        resp = requests.get(url, data={}, verify=True)
        if resp.status_code == 200:
            data = resp.json()
            return data
        raise Exception("Invalid response")

    def getTx(self, tx_hash):
        return self.checkResponse("tx", tx_hash)


# --------------------------------------------------
# Helper functions

def GetFeesInBlock(block):
    fees = 0
    tx_count = len(block["tx"])
    if (
            (block["height"] <= nLastPOWBlock and tx_count > 1) or
            (block["height"] > nLastPOWBlock and tx_count > 2)
    ):  # skip coinbase/coinstake
        for i in range(1, tx_count):
            tx_id = block["tx"][i]
            fees += int(bb_conn.getTx(tx_id).get('fees'))
    return fees/1e8

# --------------------------------------------------
# Constants

key_to_int = {
    "denom_1": 1,
    "denom_5": 5,
    "denom_10": 10,
    "denom_50": 50,
    "denom_100": 100,
    "denom_500": 500,
    "denom_1000": 1000,
    "denom_5000": 5000
}

key_to_key = {
    "denom_1": "Number of 1-denom mints",
    "denom_5": "Number of 5-denom mints",
    "denom_10": "Number of 10-denom mints",
    "denom_50": "Number of 50-denom mints",
    "denom_100": "Number of 100-denom mints",
    "denom_500": "Number of 500-denom mints",
    "denom_1000": "Number of 1000-denom mints",
    "denom_5000": "Number of 5000-denom mints"
}

ZC_DENOMS = [1, 5, 10, 50, 100, 500, 1000, 5000]

# --------------------------------------------------

# Initialize RPC connection and API client
httpConnection = httplib.HTTPConnection(rpc_host, rpc_port, timeout=20)
conn = AuthServiceProxy(rpc_url, timeout=1000, connection=httpConnection)
bb_conn = ApiClient()

# Read data from file
try:
    with open(supply_data_file, 'r') as f:
        supply_data = json.load(f)

    with open(network_data_file, 'r') as f:
        network_data = json.load(f)

except FileNotFoundError:
    # first run
    supply_data = {}
    supply_data["blocks_axis"] = []
    supply_data["time_axis"] = []
    supply_data["lastBlockNum"] = 0
    supply_data["lastBlockHash"] = ''
    supply_data["pivSupply"] = []
    zpivSupply = {}
    zpivMints = {}
    for k in ZC_DENOMS:
        denom_key = "denom_%d" % k
        zpivSupply[denom_key] = []
        zpivMints[denom_key] = []
    supply_data["zpivSupply"] = zpivSupply
    supply_data["zpivMints"] = zpivMints

    network_data = {}
    network_data["blocks_axis"] = []
    network_data["time_axis"] = []
    network_data["txs"] = []
    network_data["size"] = []
    network_data["blocktime"] = []
    network_data["difficulty"] = []
    network_data["fees"] = []



# Check if a reorg occurred
if (
        len(supply_data["blocks_axis"]) > 3 and
        conn.getblockhash(supply_data["lastBlockNum"]) != supply_data["lastBlockHash"]
):
    # remove 3 datapoints to be extra safe
    for data_key in supply_data:
        if data_key in ["zpivSupply", "zpivMints"]:
            for denom_key in supply_data[data_key]:
                supply_data[data_key][denom_key] = supply_data[data_key][denom_key][:-3]
        elif data_key != "lastBlockNum":
            supply_data[data_key] = supply_data[data_key][:-3]
        else:
            supply_data[data_key] -= 300

    for data_key in network_data:
        network_data[data_key] = network_data[data_key][:-3]


# Add new data points
blockCount = conn.getblockcount()

while supply_data["lastBlockNum"] + 100 <= blockCount:
    tx_count = 0
    fees = 0
    block_time = []
    for i in range(1, 99):
        # get intermediate block
        hash = conn.getblockhash(supply_data["lastBlockNum"] + i)
        inner_block = conn.getblock(hash, True)
        # get tx count and blocktime (resetting last)
        tx_count += len(inner_block["tx"])
        if i == 1:
            block_time.append(0)
        else:
            block_time.append(int(inner_block["time"]) - last_time)
        last_time = int(inner_block["time"])
        # get fees - skip for now
        #fees += GetFeesInBlock(inner_block)


    supply_data["lastBlockNum"] += 100
    supply_data["lastBlockHash"] = conn.getblockhash(supply_data["lastBlockNum"])
    print("Getting block %d..." % supply_data["lastBlockNum"])
    block = conn.getblock(supply_data["lastBlockHash"], True)
    tx_count += len(block["tx"])
    block_time.append(int(block["time"])-last_time)
    #fees += GetFeesInBlock(block)

    # Compute inner_block totals/avgs and reset counters
    network_data["txs"].append(tx_count)
    tx_count = 0
    network_data["blocktime"].append(int(round(sum(block_time[1:]) / len(block_time[1:]))))
    block_time = []
    network_data["fees"] = round(fees, 8)
    fees = 0


    # get PIV supply, time,  diff, size
    supply_data["pivSupply"].append(float(block["moneysupply"]))
    supply_data["time_axis"].append(int(block["time"]))
    network_data["time_axis"].append(int(block["time"]))
    network_data["difficulty"].append(float(round(block["difficulty"], 2)))
    network_data["size"].append(int(block["size"]))

    # get zPIV supply
    for k in ZC_DENOMS:
        if supply_data["lastBlockNum"] < nFirstZPivBlock:
            supply_data["zpivSupply"]["denom_%d" % k].append(0)
            supply_data["zpivMints"]["denom_%d" % k].append(0)
            continue

        supply_data["zpivSupply"]["denom_%d" % k].append(int(block["zPIVsupply"][str(k)]))
        # get mints in range
        supply_data["zpivMints"]["denom_%d" % k].append(
            int(conn.getmintsinblocks(supply_data["lastBlockNum"] - 99, 100, k)["Number of %d-denom mints" % k])
        )

    # update range
    supply_data["blocks_axis"].append(supply_data["lastBlockNum"])
    network_data["blocks_axis"].append(supply_data["lastBlockNum"])

try:
    # Save to files
    with open(supply_data_file, 'w+') as f:
        json.dump(supply_data, f)

    with open(network_data_file, 'w+') as f:
        json.dump(network_data, f)

    # close connection
    conn.close()

except Exception as e:
    print(e)
