// styling
const COLORS = {
    "green1": 'rgba(0, 255, 0, 0.8)',
    "green2": 'rgba(0, 255, 0, 0.3)',
    "red1": 'rgba(255, 0, 0, 0.8)',
    "red2": 'rgba(255, 0, 0, 0.3)',
    "purple1": 'rgba(102, 0, 153, 0.8)',
    "purple2": 'rgba(102, 0, 153, 0.3)',
};

// github data
const github_data = JSON.parse(github_data_Json);

// charts DOM elements
const commits_ctx = document.getElementById("canv_git_01_ctx");
const people_ctx = document.getElementById("canv_git_02_ctx");

// charts objects
var commitsChart;
var peopleChart;

// returns commitsChart dataPoints
function GetCommitsDataPoints() {
    var rangeObj = {
        weeks_axis: github_data.weeks_axis,
        commits: github_data.commits,
        pr_opened: github_data.pulls_opened,
        pr_merged: github_data.pulls_merged,
        pr_closed: github_data.pulls_closed
    };

    return rangeObj;
}

// returns peopleChart dataPoints
function GetPeopleDataPoints() {
    var rangeObj = {
        weeks_axis: github_data.weeks_axis,
        forks: github_data.forks,
        stars: github_data.stars,
        subscribers: github_data.subscribers,
        contributors: github_data.pull_request_contributors
    };

    return rangeObj;
}

// sets commitsChart range (all)
function SetCommitsChartRange() {
    // get new dataPoints
    const rangeObj = GetCommitsDataPoints();
    // update chart
    commitsChart.data.labels = rangeObj.weeks_axis.map(
        x => new Date(x*1000).toLocaleDateString()
    );
    commitsChart.data.datasets[0].data = rangeObj.commits;
    commitsChart.data.datasets[1].data = rangeObj.pr_opened;
    commitsChart.data.datasets[2].data = rangeObj.pr_merged;
    commitsChart.data.datasets[3].data = rangeObj.pr_closed;
    // draw chart
    const canvID = commits_ctx.id.substring(0, commits_ctx.id.length-4);
    ShowCanvas(canvID);
    commitsChart.update();
}

// sets peopleChart range (all)
function SetPeopleChartRange() {
    // get new dataPoints
    const rangeObj = GetPeopleDataPoints();
    // update chart
    peopleChart.data.labels = rangeObj.weeks_axis.map(
        x => new Date(x*1000).toLocaleDateString()
    );
    peopleChart.data.datasets[0].data = rangeObj.forks;
    peopleChart.data.datasets[1].data = rangeObj.stars;
    peopleChart.data.datasets[2].data = rangeObj.subscribers;
    peopleChart.data.datasets[3].data = rangeObj.contributors;
    // draw chart
    const canvID = people_ctx.id.substring(0, people_ctx.id.length-4);
    ShowCanvas(canvID);
    peopleChart.update();
}

// WINDOWS ON LOAD
window.onload = function () {
    InitAllCharts();
}

// --- INIT FUNCTIONS ---

// Initialize charts with whole range
function InitAllCharts() {
    InitCommitsChart();
    InitPeopleChart();
}

function InitCommitsChart() {
    let legend1 = 'No. of commits';
    let legend2 = 'No. of pull requests';
    commitsChart = InitBarChart(commits_ctx, [legend1, legend2]);
    commitsChart.data.datasets = [
        {
            data: [],
            label: "commits",
            borderColor: "gray",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line',
            yAxisID: 'axis0'
        },
        {
            data: [],
            label: "Opened PR",
            yAxisID: 'axis1',
            borderColor: COLORS.green1,
            backgroundColor: COLORS.green2,
            borderWidth: 1
        },
        {
            data: [],
            label: "Merged PR",
            yAxisID: 'axis1',
            borderColor: COLORS.purple1,
            backgroundColor: COLORS.purple2,
            borderWidth: 1
        },
        {
            data: [],
            label: "Closed PR",
            yAxisID: 'axis1',
            borderColor: COLORS.red1,
            backgroundColor: COLORS.red2,
            borderWidth: 1
        }
    ];
    SetCommitsChartRange();
}

function InitPeopleChart() {
    peopleChart = InitLineChart(people_ctx);
    peopleChart.data.datasets = [
        {
            data: [],
            label: "forks",
            borderColor: "red",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line'
        },
        {
            data: [],
            label: "stars",
            borderColor: "orange",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line'
        },
        {
            data: [],
            label: "subscribers",
            borderColor: "green",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line'
        },
        {
            data: [],
            label: "contributors",
            borderColor: "blue",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line'
        },
    ];
    SetPeopleChartRange();
}
