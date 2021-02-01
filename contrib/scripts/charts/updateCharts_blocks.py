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

# Initialize RPC connection and API client
httpConnection = httplib.HTTPConnection(rpc_host, rpc_port, timeout=20)
conn = AuthServiceProxy(rpc_url, timeout=1000, connection=httpConnection)

# Read data from file
try:
    with open(supply_data_file, 'r') as f:
        supply_data = json.load(f)

    with open(network_data_file, 'r') as f:
        network_data = json.load(f)

except FileNotFoundError:
    # first run (hardcoded)
    print("First run...")
    supply_data = {}
    supply_data["blocks_axis"] = [0]
    supply_data["time_axis"] = [0]
    supply_data["shield_supply"] = [0]

    network_data = {}
    network_data["blocks_axis"] = [0]
    network_data["time_axis"] = [0]
    network_data["difficulty"] = [0]
    network_data["blocktime"] = [0]
    network_data["blocksize"] = [0]
    network_data["txs"] = [0]
    network_data["fees_ttl"] = [0]
    network_data["fees_perKb"] = [0]

# Check if a reorg occurred
last_block_hash = conn.getblockhash(supply_data["blocks_axis"][-1])
if (
        len(supply_data["blocks_axis"]) > 6 and
        last_block_hash != supply_data["lastBlockHash"]
):
    # remove 3 datapoints to be extra safe
    for data_key in ["blocks_axis", "time_axis", "shield_supply"]:
        supply_data[data_key] = supply_data[data_key][:-3]

    for data_key in network_data:
        network_data[data_key] = network_data[data_key][:-3]


# Add new data points
blockCount = conn.getblockcount()

while supply_data["blocks_axis"][-1] + 100 <= blockCount:
    # fetch block N+100
    new_block_num = supply_data["blocks_axis"][-1] + 100
    supply_data["lastBlockHash"] = conn.getblockhash(new_block_num)
    print("Getting block %d..." % new_block_num)
    block = conn.getblock(supply_data["lastBlockHash"], True)
    blockheader = conn.getblockheader(supply_data["lastBlockHash"], True)

    # get time, blocktime, blocksize and difficulty
    supply_data["time_axis"].append(int(block["time"]))
    network_data["time_axis"].append(int(block["time"]))
    network_data["difficulty"].append(float(round(block["difficulty"], 2)))
    network_data["blocktime"].append((network_data["time_axis"][-1]-network_data["time_axis"][-2])/100)
    network_data["blocksize"].append(int(block["size"]))

    # fetch blockindexstats over 100 blocks: [N, N+99]
    block_stats = conn.getblockindexstats(new_block_num-100, 100)

    # get shield supply
    supply_data["shield_supply"].append(float(blockheader["shield_pool_value"]["chainValue"]))

    # get tx count and fees
    network_data["txs"].append(int(block_stats["txcount_all"]))
    network_data["fees_ttl"].append(float(block_stats["ttlfee_all"]))
    network_data["fees_perKb"].append(float(block_stats["feeperkb"]))

    # update range
    supply_data["blocks_axis"].append(new_block_num)
    network_data["blocks_axis"].append(new_block_num)

try:
    # Save to files
    with open(supply_data_file, 'w+') as f:
        json.dump(supply_data, f)

    with open(network_data_file, 'w+') as f:
        json.dump(network_data, f)

    # close connection
    httpConnection.close()

except Exception as e:
    print(e)
