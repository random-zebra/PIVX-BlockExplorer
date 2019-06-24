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
# Constants

ZC_DENOMS = [1, 5, 10, 50, 100, 500, 1000, 5000]

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
    # first run (blocks 0-99 and 100-199 hardcoded)
    supply_data = {}
    supply_data["blocks_axis"] = [0, 100]
    if isTestnet:
        supply_data["time_axis"] = [1454124731, 1488951557]
        supply_data["lastBlockHash"] = '0000041e482b9b9691d98eefb48473405c0b8ec31b76df3797c74a78680ef818'
        supply_data["pivSupply"] = [0.0, 24810000.99975790]
    else:
        supply_data["time_axis"] = [1454124731, 1454186818]
        supply_data["lastBlockHash"] = '0000000a86f23294329c83d69e254a4f8d127b6b899a14b147885740c4be1713'
        supply_data["pivSupply"] = [0.0, 84751.0]
    zpivSupply = {}
    zpivMints = {}
    for k in ZC_DENOMS:
        denom_key = "denom_%d" % k
        zpivSupply[denom_key] = [0, 0]
        zpivMints[denom_key] = [0, 0]
    supply_data["zpivSupply"] = zpivSupply
    supply_data["zpivMints"] = zpivMints

    network_data = {}
    network_data["blocks_axis"] = [0, 100]
    if isTestnet:
        network_data["time_axis"] = [1454124731, 1488951557]
        network_data["txs"] = [0, 107]
        network_data["fees_ttl"] = [0, 0.0003067]
        network_data["fees_perKb"] = [0, 0.00012864]
        network_data["blocksize"] = [0, 214]
        network_data["blocktime"] = [0, (1488951557-1454124731)/100]
        network_data["difficulty"] = [0.00, 0.00]
    else:
        network_data["time_axis"] = [1454124731, 1454186818]
        network_data["txs"] = [0, 100]
        network_data["fees_ttl"] = [0, 0.0027982]
        network_data["fees_perKb"] = [0, 0.00011946]
        network_data["blocksize"] = [0, 196]
        network_data["blocktime"] = [0, (1454186818-1454124731)/100]
        network_data["difficulty"] = [0.00, 0.09]


# Check if a reorg occurred
last_block_hash = conn.getblockhash(supply_data["blocks_axis"][-1])
if (
        len(supply_data["blocks_axis"]) > 6 and
        last_block_hash != supply_data["lastBlockHash"]
):
    # remove 3 datapoints to be extra safe
    supply_data["lastBlockHash"] = last_block_hash
    for data_key in ["zpivSupply", "zpivMints"]:
        for denom_key in supply_data[data_key]:
            supply_data[data_key][denom_key] = supply_data[data_key][denom_key][:-3]
    for data_key in ["blocks_axis", "time_axis", "pivSupply"]:
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

    # get PIV supply and zPIV supply
    supply_data["pivSupply"].append(float(block["moneysupply"]))
    for k in ZC_DENOMS:
        supply_data["zpivSupply"]["denom_%d" % k].append(int(block["zPIVsupply"][str(k)]))

    # get time, blocktime, blocksize and difficulty
    supply_data["time_axis"].append(int(block["time"]))
    network_data["time_axis"].append(int(block["time"]))
    network_data["difficulty"].append(float(round(block["difficulty"], 2)))
    network_data["blocktime"].append((network_data["time_axis"][-1]-network_data["time_axis"][-2])/100)
    network_data["blocksize"].append(int(block["size"]))

    # fetch blockindexstats over 100 blocks: [N, N+99]
    block_stats = conn.getblockindexstats(new_block_num-100, 100)

    # get mints
    for k in ZC_DENOMS:
        # get mints in range
        supply_data["zpivMints"]["denom_%d" % k].append(int(block_stats["mintcount"]["denom_%d" % k]))

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
