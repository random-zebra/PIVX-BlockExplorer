// blocks data
const block_data = JSON.parse(network_data_Json);
const LAST_BLOCK_NUM = block_data.blocks_axis[block_data.blocks_axis.length-1];
let fees_byte = [];

// charts DOM elements
const difficulty_ctx = document.getElementById("canv_net_01_ctx");
const blocksize_ctx = document.getElementById("canv_net_02_ctx");
const fee_ctx = document.getElementById("canv_net_03_ctx");

// charts objects
var difficultyChart, blocksizeChart, feeChart;

function AddBlockSizeToolTip() {
    blocksizeChart.options.tooltips.callbacks.label = function(tooltipItem, data) {
        let this_dataset = data.datasets[tooltipItem.datasetIndex];
        if (this_dataset.yAxisID == "axis0") {
            return this_dataset.label + ": " + tooltipItem.value + ' B';
        } else if (this_dataset.yAxisID == "axis1") {
            let label = AmountToDecimal(tooltipItem.value , 2);
            return this_dataset.label + ": " + label + ' tx/block';
        } else {
            return this_dataset.label + ": " + tooltipItem.value
        }
    }
}

// returns difficultyChart dataPoints for range (x-axis)
function GetDiffDataPoints(bl_from, bl_to) {
    // range object used for dataSets - initialize empty
    var rangeObj = {
        blocks_axis: [],
        time_axis: [],
        difficulty: [],
        blocktime: []
    };

    // Limit to ~120 points
    const step = 1+Math.floor((bl_to-bl_from) / 120);

    // Skip some points. Add total per-block data
    avg_blocktime = 0
    counted_blocks = 0

    for (let i = bl_from; i <= bl_to; i++) {
        avg_blocktime += block_data.blocktime[i];
        counted_blocks++;
        if (step <= 1 || i % step == 0) {
            // add data to rangeObj
            rangeObj.blocks_axis.push(block_data.blocks_axis[i]);
            rangeObj.time_axis.push(new Date(block_data.time_axis[i]*1000).toLocaleString());
            rangeObj.difficulty.push(block_data.difficulty[i]);
            rangeObj.blocktime.push(avg_blocktime / counted_blocks);
            // reset block-range objects sums
            avg_blocktime = 0
            counted_blocks = 0
        }
    }
    return rangeObj;
}

// returns blocksizeChart dataPoints for range (x-axis)
function GetBlocksizeDataPoints(bl_from, bl_to) {
    // range object used for dataSets - initialize empty
    var rangeObj = {
        blocks_axis: [],
        time_axis: [],
        blocksize: [],
        txes: [],
        tot_txes: []
    };

    // Limit to ~120 points
    const step = 1+Math.floor((bl_to-bl_from) / 120);

    let tx_count = 0;
    let tot_tx_count = 0;
    for (let i = bl_from; i <= bl_to; i++) {
        tx_count += block_data.txs[i];
        tot_tx_count += block_data.txs[i];
        if (step <= 1 || i % step == 0) {
            // add data to rangeObj
            rangeObj.blocks_axis.push(block_data.blocks_axis[i]);
            rangeObj.time_axis.push(new Date(block_data.time_axis[i]*1000).toLocaleString());
            rangeObj.blocksize.push(block_data.size[i]);
            rangeObj.txes.push(tx_count/(100*step));
            rangeObj.tot_txes.push(tot_tx_count);
            tx_count = 0;
        }
    }

    return rangeObj;
}

// returns feeChart dataPoints for range (x-axis)
function GetFeeDataPoints(bl_from, bl_to) {
    // range object used for dataSets - initialize empty
    var rangeObj = {
        blocks_axis: [],
        time_axis: [],
        block_fees: [],
        tot_fees: [],
        avg_fees: [],
    };

    // Limit to ~120 points
    const step = 1+Math.floor((bl_to-bl_from) / 120);

    let tot_fees = 0;
    for (let i = bl_from; i <= bl_to; i++) {
        tot_fees += block_data.fees[i];
        if (step <= 1 || i % step == 0) {
            // add data to rangeObj
            rangeObj.blocks_axis.push(block_data.blocks_axis[i]);
            rangeObj.time_axis.push(new Date(block_data.time_axis[i]*1000).toLocaleString());
            rangeObj.block_fees.push(block_data.fees[i]);
            rangeObj.tot_fees.push(AmountToDecimal(tot_fees, 8));
            rangeObj.avg_fees.push(fees_byte[i]);
        }
    }
    return rangeObj;
}

// sets difficultyChart range and updates it
function SetDiffChartRange(val_from, val_to) {
    // get new dataPoints
    const rangeObj = GetDiffDataPoints(val_from, val_to);
    // update chart
    SetDataLabel(difficultyChart, rangeObj);
    difficultyChart.data.datasets[0].data = rangeObj.difficulty;
    difficultyChart.data.datasets[1].data = rangeObj.blocktime;
    // draw chart
    const canvID = difficulty_ctx.id.substring(0, difficulty_ctx.id.length-4);
    ShowCanvas(canvID);
    difficultyChart.update();
}

// sets blocksizeChart range and updates it
function SetBlockSizeChartRange(val_from, val_to) {
    // get new dataPoints
    const rangeObj = GetBlocksizeDataPoints(val_from, val_to);
     // update chart
    SetDataLabel(blocksizeChart, rangeObj);
    blocksizeChart.data.datasets[0].data = rangeObj.blocksize;
    blocksizeChart.data.datasets[1].data = rangeObj.txes;
    blocksizeChart.data.datasets[2].data = rangeObj.tot_txes;
    // draw chart
    const canvID = blocksize_ctx.id.substring(0, blocksize_ctx.id.length-4);
    ShowCanvas(canvID);
    blocksizeChart.update();
}

// sets feeChart range and updates it
function SetFeeChartRange(val_from, val_to) {
    // get new dataPoints
    const rangeObj = GetFeeDataPoints(val_from, val_to);
     // update chart
    SetDataLabel(feeChart, rangeObj);
    feeChart.data.datasets[0].data = rangeObj.avg_fees;
    feeChart.data.datasets[1].data = rangeObj.block_fees;
    feeChart.data.datasets[2].data = rangeObj.tot_fees;
    // draw chart
    const canvID = fee_ctx.id.substring(0, fee_ctx.id.length-4);
    ShowCanvas(canvID);
    feeChart.update()
}

// WINDOWS ON LOAD
window.onload = function () {
    ComputeAverages();
    InitAllCharts();
}

// --- INIT FUNCTIONS ---

function ComputeAverages() {
    for (let i = 0; i < block_data.blocks_axis.length; i++) {
        if (block_data.txs > 1) {
            fees_byte.push(AmountToDecimal(block_data.fees / (block_data.size - 180), 8));
        } else {
            fees_byte.push(0);
        }
    }
}

// Initialize charts with whole range
function InitAllCharts() {
    InitDiffChart();
    InitBlockSizeChart();
    InitFeeChart();
}

function InitDiffChart() {
    let legend1 = 'Difficulty';
    let legend2 = 'Avg. Blocktime (sec)';
    difficultyChart = InitLineChart(difficulty_ctx, [legend1, legend2]);
    difficultyChart.options.tooltips.callbacks.title = tooltipTitle;
    difficultyChart.data.labelset = "blocks";
    difficultyChart.data.datasets = [
        {
            data: [],
            label: "difficulty",
            borderColor: "rgba(102, 51, 204, 0.7)",
            backgroundColor: "rgba(102, 51, 204, 0.3)",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: true,
            yAxisID: 'axis0'
        },
        {
            data: [],
            label: "avg. blocktime",
            borderColor: "rgba(51, 153, 102, 0.7)",
            fill: false,
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            yAxisID: 'axis1'
        }
    ];
    difficultyChart.options.onResize = MinimizeChartLegend;
    SetDiffChartRange(1, (block_data.blocks_axis.length - 1));
}

function InitBlockSizeChart() {
    let legend1 = 'Avg. Blocksize (bytes)';
    let legend2 = 'Avg. no. of txes per block';
    let legend3 = 'TOT no. of txes'
    blocksizeChart = InitBarChart(blocksize_ctx, [legend1, legend2, legend3]);
    blocksizeChart.options.tooltips.callbacks.title = tooltipTitle;
    blocksizeChart.data.labelset = "blocks"
    blocksizeChart.data.datasets = [
        {
            data: [],
            label: "avg. blocksize",
            borderColor: "orange",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line',
            yAxisID: 'axis0'
        },
        {
            data: [],
            label: "avg. tx/block",
            yAxisID: 'axis1',
            borderColor: 'rgba(173, 244, 66, 0.7)',
            backgroundColor: 'rgba(173, 244, 66, 0.3)',
            borderWidth: 1
        },
        {
            data: [],
            label: "total tx count",
            borderColor: 'rgba(66, 244, 194, 0.8)',
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line',
            yAxisID: 'axis2'
        },
    ];
    blocksizeChart.options.onResize = MinimizeChartLegend;
    AddBlockSizeToolTip();
    SetBlockSizeChartRange(1, (block_data.blocks_axis.length - 1));
}

function InitFeeChart() {
    let legend1 = 'Avg. Fee per byte (PIV)';
    let legend2 = 'Fees in blocks (PIV)';
    let legend3 = 'TOT Burnt Fees (PIV)'
    feeChart = InitBarChart(fee_ctx, [legend1, legend2, legend3]);
    feeChart.options.tooltips.callbacks.title = tooltipTitle;
    feeChart.data.labelset = "blocks"
    feeChart.data.datasets = [
        {
            data: [],
            label: "avg. PIV/byte fee",
            borderColor: "rgba(244, 66, 83, 0.8)",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line',
            yAxisID: 'axis0'
        },
        {
            data: [],
            label: "fees in blocks",
            yAxisID: 'axis1',
            borderColor: 'rgba(244, 217, 65, 0.7)',
            backgroundColor: 'rgba(244, 217, 65, 0.3)',
            borderWidth: 1
        },
        {
            data: [],
            label: "total fees",
            borderColor: 'rgba(167, 66, 244, 0.8)',
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            type: 'line',
            yAxisID: 'axis2'
        },
    ];
    feeChart.options.onResize = MinimizeChartLegend;
    SetFeeChartRange(1, (block_data.blocks_axis.length - 1));
}

// map elements ID of canvas headers to right 'SetChartRange' function
function MapIdToSetChartRange(canv_id, val_from, val_to) {
    switch(canv_id) {
        case "canv_net_01":
            return SetDiffChartRange(val_from, val_to);
        case "canv_net_02":
            return SetBlockSizeChartRange(val_from, val_to);
        case "canv_net_03":
            return SetFeeChartRange(val_from, val_to);
        default:
            alert(canv_id + " not found");
            return;
    }
}

// map elements ID of canvas headers to right chart
function MapIdToChart(canv_id) {
    switch(canv_id) {
        case "canv_net_01":
            return difficultyChart;
        case "canv_net_02":
            return blocksizeChart;
        case "canv_net_03":
            return feeChart;
        default:
            return difficultyChart;
    }
};
