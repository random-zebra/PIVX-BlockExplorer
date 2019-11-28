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

# FILENAME
mn_data_file = os.path.join("/opt/coins/blockbook/plot_data", "mn_data.json")

# CONSTANTS
LAST_POW_BLOCK = 200 if isTestnet else 259200

def getPaidMasternode(block_hash):
    try:
        block = conn.getblock(block_hash, True)
        coinstake = conn.getrawtransaction(block['tx'][1], True)
        return coinstake['vout'][-1]['scriptPubKey']['addresses'][0], block['time']
    except Exception as e:
        print(str(e))
        return "", 0

# Initialize RPC connection
httpConnection = httplib.HTTPConnection(rpc_host, rpc_port, timeout=20)
conn = AuthServiceProxy(rpc_url, timeout=1000, connection=httpConnection)

# Read data from file
try:
    with open(mn_data_file, 'r') as f:
        mn_data = json.load(f)

except FileNotFoundError:
    # first run
    mn_data = {}
    mn_data["lastBlockHash"] = ""
    mn_data["time_axis"] = [0]
    mn_data["double_mn_payments"] = [0]    # duble payments occurred in range

# Add new data points
blockCount = conn.getblockcount()
fetching_block_num = len(mn_data["double_mn_payments"]) - 1
double_payments = mn_data["double_mn_payments"][-1]
last_mn_paid = ""

while fetching_block_num < blockCount:
    fetching_block_num += 1
    if fetching_block_num <= LAST_POW_BLOCK:
        mn_data["time_axis"].append(0)
        mn_data["double_mn_payments"].append(0)
        mn_data["lastBlockHash"] = ""
        continue
    print("Getting masternode paid at block %d..." % fetching_block_num)
    fetching_block_hash = conn.getblockhash(fetching_block_num)
    mn_paid, time = getPaidMasternode(fetching_block_hash)
    if mn_paid != "":
        if mn_paid == last_mn_paid:
            double_payments += 1
        else:
            last_mn_paid = mn_paid
    mn_data["time_axis"].append(time)
    mn_data["double_mn_payments"].append(double_payments)
    mn_data["lastBlockHash"] = fetching_block_hash

    if fetching_block_num % 100000 == 0:
        print("Saving...")
        try:
            # Save to files
            with open(mn_data_file, 'w+') as f:
                json.dump(mn_data, f)
        except Exception as e:
            print(str(e))

print("Finally Saving...")
try:
    # Save to files
    with open(mn_data_file, 'w+') as f:
        json.dump(mn_data, f)

    # close connection
    httpConnection.close()

except Exception as e:
    print(e)
