#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import datetime
import json
import os
import requests

# FILENAME
github_data_file = os.path.join("/opt/coins/blockbook/plot_data", "github_data.json")

# --------------------------------------------------
class GithubClient:

    def __init__(self):
        self.url = "https://api.github.com"

    def checkResponse(self, repo, method, param=""):
        url = self.url + "/repos/PIVX-Project/%s/%s" % (repo, method)
        if param != "":
            url += "/%s" % param
        resp = requests.get(url, data={}, verify=True)
        if resp.status_code == 200:
            data = resp.json()
            return data
        raise Exception("Invalid response: %s" % str(resp))

    def getContributors(self, repo='PIVX'):
        return self.checkResponse(repo, "stats/contributors")

    def getPulls(self, sorting='created', repo='PIVX'):
        pulls = []
        try:
            i = 1
            while True:
                new_page = self.checkResponse(repo, "pulls?state=all&sort=%s&direction=desc&page=%d" % (sorting, i))
                if len(new_page) == 0:
                    break
                npc = new_page[0]["closed_at"]
                if npc is not None and DateTimeToEpoch(npc) < github_data["weeks_axis"][0]:
                    break
                pulls = pulls + new_page
                i += 1
        except Exception as e:
            print(e)
        return pulls


    def getWeeklyCommits(self, repo='PIVX'):
        return self.checkResponse(repo, "stats/commit_activity")



class GeckoClient:

    def __init__(self):
        self.url = "https://api.coingecko.com/api/v3/coins/pivx"

    def checkResponse(self, method, param=""):
        url = self.url + "/%s" % method
        if param != "":
            url += "?%s" % param
        resp = requests.get(url, data={}, verify=True)
        if resp.status_code == 200:
            data = resp.json()
            return data
        raise Exception("Invalid response: %s" % str(resp))

    def getDevData(self, epoch_dates):
        ret = {}
        ret["forks"] = []
        ret["stars"] = []
        ret["subscribers"] = []
        ret["pull_request_contributors"] = []

        for w in epoch_dates:
            curr_date = EpochToDate(w)
            curr_json = self.checkResponse("history", "date=%s&localization=false" % curr_date)
            dev_data = curr_json["developer_data"]
            for dkey in ret:
                ret[dkey].append(dev_data[dkey])

        return ret

# --------------------------------------------------
# Helper functions

def AddToWeeklySum(weeks, sums, el_time):
    i = len(weeks) - 1
    while el_time < weeks[i]:
        i -= 1
        if i < 0:
            break
    if i >= 0:
        sums[i] += 1

def DateTimeToEpoch(dStr, dFormat='%Y-%m-%dT%H:%M:%Sz'):
    return int(datetime.datetime.strptime(dStr, dFormat).timestamp())

def EpochToDate(dInt, dFormat='%d-%m-%Y'):
    return datetime.datetime.fromtimestamp(dInt).strftime(dFormat)

# --------------------------------------------------

# Initialize Github and Gecko API client
git_conn = GithubClient()
gecko_conn = GeckoClient()

# Init return object - week_51 is current week. week_0 is 51 weeks ago.
github_data = {}
github_data["weeks_axis"] = []
github_data["commits"] = []

# get commits and axis
commits = git_conn.getWeeklyCommits()
github_data["weeks_axis"] = [c["week"] for c in commits]
github_data["commits"] = [c["total"] for c in commits]
tot_len = len(github_data["weeks_axis"])

# get pulls
pulls = git_conn.getPulls()
github_data["pulls_opened"] = [0] * tot_len
github_data["pulls_merged"] = [0] * tot_len
github_data["pulls_closed"] = [0] * tot_len

for op in pulls:
    pt_opened = DateTimeToEpoch(op["created_at"])
    AddToWeeklySum(github_data["weeks_axis"], github_data["pulls_opened"], pt_opened)

    pt_merged = op["merged_at"]
    if pt_merged is not None:
        pt_merged = DateTimeToEpoch(pt_merged)
        AddToWeeklySum(github_data["weeks_axis"], github_data["pulls_merged"], pt_merged)

    pt_closed = op["closed_at"]
    if pt_merged is None and pt_closed is not None:
        pt_closed = DateTimeToEpoch(pt_closed)
        AddToWeeklySum(github_data["weeks_axis"], github_data["pulls_closed"], pt_closed)

# get gecko data ("forks", "stars", "subscribers", "pull_request_contributors")
gecko = gecko_conn.getDevData(github_data["weeks_axis"])
for dkey in gecko:
    github_data[dkey] = gecko[dkey]



try:
    # Save to files
    with open(github_data_file, 'w') as f:
        json.dump(github_data, f)

except Exception as e:
    print(e)

print("Updated last week: %s" % datetime.datetime.fromtimestamp(
    github_data["weeks_axis"][tot_len-1]
).strftime('%Y-%m-%d %H:%M:%S'))
