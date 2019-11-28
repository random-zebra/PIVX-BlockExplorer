// testing data
const block_data = JSON.parse(testing_data_Json)
const LAST_BLOCK_NUM = block_data.double_mn_payments.length - 1;

// charts DOM elements
const double_mnpayments_ctx = document.getElementById("canv_testing_01_ctx")

// charts objects
var doubleMNpaymentsChart;

function GetMNDataPoints(bl_from, bl_to) {
  var rangeObj = {
    blocks_axis: [],
    time_axis: [],
    total_payments: [],
    average: [],
    frequency: []
  };

  // Limit to ~120 points
  const step = 1+Math.floor((bl_to-bl_from) / 120);

  for (let i = bl_from; i <= bl_to; i++) {
      if (step <= 1 || i % step == 0) {
          // add data to rangeObj
          rangeObj.blocks_axis.push(i);
          rangeObj.time_axis.push(new Date(block_data.time_axis[i]*1000).toLocaleString());
          rangeObj.total_payments.push(block_data.double_mn_payments[i]);
          if (i > 50000) {
            rangeObj.average.push((block_data.double_mn_payments[i] - block_data.double_mn_payments[i-50000])/50000);
          } else {
            rangeObj.average.push(0);
          }
          let totlen = rangeObj.total_payments.length;
          if (totlen > 1) {
            rangeObj.frequency.push((rangeObj.total_payments[totlen-1] - rangeObj.total_payments[totlen-2])/step);
          } else {
            rangeObj.frequency.push(0);
          }
      }
  }
  return rangeObj;
}

function SetMNChartRange(val_from, val_to) {
  // get new dataPoints
  const rangeObj = GetMNDataPoints(val_from, val_to);
  // update chart
  SetDataLabel(doubleMNpaymentsChart, rangeObj);
  doubleMNpaymentsChart.data.datasets[0].data = rangeObj.total_payments;
  doubleMNpaymentsChart.data.datasets[1].data = rangeObj.frequency;
  doubleMNpaymentsChart.data.datasets[2].data = rangeObj.average;
  // draw chart
  const canvID = double_mnpayments_ctx.id.substring(0, double_mnpayments_ctx.id.length-4);
  ShowCanvas(canvID);
  doubleMNpaymentsChart.update();
}

// WINDOWS ON LOAD
window.onload = function () {
    InitAllCharts();
}

// Initialize charts with whole range
function InitAllCharts() {
    InitMNChart();
}

function InitMNChart() {
    let legend1 = 'TOT double payments';
    let legend2 = 'average';
    doubleMNpaymentsChart = InitLineChart(double_mnpayments_ctx, [legend1, legend2]);
    doubleMNpaymentsChart.options.tooltips.callbacks.title = tooltipTitle;
    doubleMNpaymentsChart.data.labelset = "blocks";
    doubleMNpaymentsChart.data.datasets = [
        {
            data: [],
            label: "tot",
            borderColor: "rgba(102, 51, 204, 0.7)",
            backgroundColor: "rgba(102, 51, 204, 0.3)",
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            fill: true,
            yAxisID: 'axis0'
        },
        {
            data: [],
            label: "frequency",
            borderColor: "rgba(0, 15, 102, 0.7)",
            fill: false,
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            yAxisID: 'axis1'
        },
        {
            data: [],
            label: "50K-blocks MA",
            borderColor: "rgba(51, 153, 102, 0.7)",
            fill: false,
            pointRadius: 2,
            needsRadius: 2,   // for resize,
            yAxisID: 'axis1'
        }
    ];
    doubleMNpaymentsChart.options.onResize = MinimizeChartLegend;
    SetMNChartRange(1, (block_data.double_mn_payments.length - 1));
}

// map elements ID of canvas headers to right 'SetChartRange' function
function MapIdToSetChartRange(canv_id, val_from, val_to) {
    // only one chart for now
    return SetMNChartRange(val_from, val_to)
}

// map elements ID of canvas headers to right chart
function MapIdToChart(canv_id) {
    // only one chart for now
    return doubleMNpaymentsChart;
};
