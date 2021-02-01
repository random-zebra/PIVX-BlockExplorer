// styling
const SHIELD_COLOR = 'rgba(51, 102, 153, 0.7)'

// blocks data
const block_data = JSON.parse(supply_data_Json);
const LAST_BLOCK_NUM = block_data.blocks_axis[block_data.blocks_axis.length-1];
const supplydata = block_data.shield_supply;

// charts DOM elements
const supply_ctx = document.getElementById("canv_supply_ctx");

// charts objects
var supplyChart;

function AddSupplyToolTip(chart) {
    chart.options.tooltips.callbacks.label = function(tooltipItem, data) {
        let this_dataset = data.datasets[tooltipItem.datasetIndex];
        // axis0 return millions rounded to two decimal places
        if (this_dataset.yAxisID == "axis0") {
            let label = Math.round(tooltipItem.value / 10000) / 100;
            return this_dataset.label + ": " + label + 'M PIV';
        } else {
            // for axis1 return regular label
            return this_dataset.label + ": " + tooltipItem.value;
        }
    }
}

// returns shieldsupplyChart dataPoints for range (x-axis)
function GetSupplyDataPoints(bl_from, bl_to) {
    // range object used for dataSets - initialize empty
    var rangeObj = {
        blocks_axis: [],
        time_axis: [],
        shield_supply: [],
    };

    // Limit to ~120 points
    const step = 1+Math.floor((bl_to-bl_from) / 120);

    for (let i = bl_from; i <= bl_to; i++) {
        if (step <= 1 || i % step == 0) {
            // add data to rangeObj
            rangeObj.blocks_axis.push(block_data.blocks_axis[i]);
            rangeObj.time_axis.push(new Date(block_data.time_axis[i]*1000).toLocaleString());
            rangeObj.shield_supply.push(block_data.shield_supply[i]);
        }
    }
    return rangeObj;
}

// sets shieldsupplyChart range and updates it
function SetSupplyChartRange(val_from, val_to) {
    // get new dataPoints
    const rangeObj = GetSupplyDataPoints(val_from, val_to);
    // update chart
    SetDataLabel(supplyChart, rangeObj);
    supplyChart.data.datasets[0].data = rangeObj.shield_supply;
    // draw chart
    const canvID = supply_ctx.id.substring(0, supply_ctx.id.length-4);
    ShowCanvas(canvID);
    supplyChart.update()
}

// WINDOWS ON LOAD
window.onload = function () {
    InitAllCharts();
}

// --- INIT FUNCTIONS ---

// Initialize charts with whole range
function InitAllCharts() {
    InitSupplyChart();
}

function InitSupplyChart() {
    let legend = 'Shield Supply';
    let pointradius = 2;
    supplyChart = InitLineChart(supply_ctx, [legend]);
    supplyChart.options.tooltips.callbacks.title = tooltipTitle;
    supplyChart.data.labelset = "blocks";
    supplyChart.data.datasets = [
        {
            data: [],
            label: "Shield Supply",
            borderColor: SHIELD_COLOR,
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: false,
            yAxisID: 'axis0'
        }
    ];

    supplyChart.options.onResize = MinimizeChartLegend;
    AddSupplyToolTip(supplyChart);
    SetSupplyChartRange(1, (block_data.blocks_axis.length - 1));
}

// map elements ID of canvas headers to right 'SetChartRange' function
function MapIdToSetChartRange(canv_id, val_from, val_to) {
    switch(canv_id) {
        case "canv_supply":
            return SetSupplyChartRange(val_from, val_to);
        default:
            alert(canv_id + " not found");
            return;
    }
}

// map elements ID of canvas headers to right chart
function MapIdToChart(canv_id) {
    switch(canv_id) {
        default:
            return supplyChart;
    }
};
