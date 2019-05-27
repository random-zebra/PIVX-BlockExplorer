#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from bitcoinrpc.authproxy import AuthServiceProxy
try:
    import http.client as httplib
except ImportError:
    import httplib
import json

# CONFIG
rpc_user = "rpc"
rpc_pass = "pivxrpc"
rpc_host = "127.0.0.1"
rpc_port = "18049"
rpc_url = "http://%s:%s@%s" % (rpc_user, rpc_pass, rpc_host)
# FIRST BLOCK
nFirstZPivBlock = 201576 - 201576 % 100 - 100

#--------------------------------------------------

# Initialize RPC connection
httpConnection = httplib.HTTPConnection(rpc_host, rpc_port, timeout=20)
conn = AuthServiceProxy(rpc_url, timeout=1000, connection=httpConnection)

# Read data from file
try:
	with open("zpivsupplydata.json", 'r') as f:
		data = json.load(f)
except FileNotFoundError:
	# first run - fill initial empty supply
	data = {}
	nEmptyElems = int(nFirstZPivBlock/100)
	data["lastBlockNum"] = nFirstZPivBlock
	data["lastBlockHash"] = conn.getblockhash(nFirstZPivBlock)
	data["denom_1"] = [0] * nEmptyElems
	data["denom_5"] = [0] * nEmptyElems
	data["denom_10"] = [0] * nEmptyElems
	data["denom_50"] = [0] * nEmptyElems
	data["denom_100"] = [0] * nEmptyElems
	data["denom_500"] = [0] * nEmptyElems
	data["denom_1000"] = [0] * nEmptyElems
	data["denom_5000"] = [0] * nEmptyElems
	data["total"] = [0] * nEmptyElems
	data["blocks_axis"] = [i*100 for i in range(nEmptyElems)]

# Check if a reorg occurred
if conn.getblockhash(data["lastBlockNum"]) != data["lastBlockHash"] and len(data["blocks_axis"]) > 3:
	# remove 3 datapoints to be extra safe
        data["lastBlockNum"] -= 300
        data["lastBlockHash"] = conn.getblockhash(data["lastBlockNum"])
        data["denom_1"] = data["denom_1"][:-3]
        data["denom_5"] = data["denom_5"][:-3]
        data["denom_10"] = data["denom_10"][:-3]
        data["denom_50"] = data["denom_50"][:-3]
        data["denom_100"] = data["denom_100"][:-3]
        data["denom_500"] = data["denom_500"][:-3]
        data["denom_1000"] = data["denom_1000"][:-3]
        data["denom_5000"] = data["denom_5000"][:-3]
        data["total"] = data["total"][:-3]
        data["blocks_axis"] = data["blocks_axis"][:-3]

# Add new data points
blockCount = conn.getblockcount()
while data["lastBlockNum"] + 100 <= blockCount:
	data["lastBlockNum"] += 100
	data["lastBlockHash"] = conn.getblockhash(data["lastBlockNum"])
	print("Getting block %d..." % data["lastBlockNum"])
	block = conn.getblock(data["lastBlockHash"], True)
	data["denom_1"].append(int(block["zPIVsupply"]["1"]))
	data["denom_5"].append(int(block["zPIVsupply"]["5"]))
	data["denom_10"].append(int(block["zPIVsupply"]["10"]))
	data["denom_50"].append(int(block["zPIVsupply"]["50"]))
	data["denom_100"].append(int(block["zPIVsupply"]["100"]))
	data["denom_500"].append(int(block["zPIVsupply"]["500"]))
	data["denom_1000"].append(int(block["zPIVsupply"]["1000"]))
	data["denom_5000"].append(int(block["zPIVsupply"]["5000"]))
	data["total"].append(int(block["zPIVsupply"]["total"]))
	data["blocks_axis"].append(data["lastBlockNum"])

# Save to file
try:
	with open("zpivsupplydata.json", 'w+') as f:
		json.dump(data, f)
except:
	pass
