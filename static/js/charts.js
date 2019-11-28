// constant steps
const STEPS_YEAR = 5256;
const STEPS_MONTH = 432;
const STEPS_WEEK = 100;
const STEPS_DAY = 15;

// INIT for charts types

// returns initialized charts
function InitBarChart(ctx, y_labels) {
    let c = new Chart(ctx, {
        type: 'bar',
        maintainAspectRatio : false,
        data: {
            labelset: "blocks",
            labels: [],
            datasets: [],
            hiddenlabels: []
        },
        options: {
            legend: {
                position: 'bottom'
            },
            tooltips: {
                position: 'nearest',
                mode: 'index',
                intersect: false
            }
        },
        plugins: [verticalLinePlugin]
    });

    if (y_labels && y_labels.length > 0) {
        let scales1 = GetScalesFromLabels(y_labels);
        scales1.xAxes = [{
            stacked: true
        }];
        if (scales1.yAxes.length > 1) {
            scales1.yAxes[1].stacked = true;
        }
        c.options.scales = scales1;
    }
    return c;
}

function InitLineChart(ctx, y_labels) {
    let c = new Chart(ctx, {
        type: 'line',
        maintainAspectRatio : false,
        data: {
            labelset: "blocks",
            labels: [],
            datasets: []
        },
        options: {
            legend: {
                position: 'bottom'
            },
            tooltips: {
                position: 'nearest',
                mode: 'index',
                intersect: false
            }
        },
        plugins: [verticalLinePlugin]
    });

    c.options.scales = GetScalesFromLabels(y_labels);

    return c;
}

function InitDoughnutChart(ctx) {
    return new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: [],
            datasets: []
        },
        options: {
            legend: {
                position: 'right'
            }
        }
    });
}

const tooltipTitle = function(tooltipItem, data) {
    let blocknum = data.labels[tooltipItem[0].index]
    let date = data.hiddenlabels[tooltipItem[0].index]
    if (data.labelset == "blocks") {
        return "Block: " + blocknum + '\n(' + date + ')';
    } else {
        return blocknum + '\n(Block: ' + date + ')';
    }
}

// HELPERS

// returns amount rounded to precision digits
function AmountToDecimal(amount, precision) {
    if (amount == 0) {
        return 0;
    }
    const pow10 = 10 ** precision;
    return Math.round(amount * pow10) / pow10;
}

// returns time range for form-value range
function GetTimeRange(range, step) {
    switch (range) {
        case "last_year":
            return Math.round(STEPS_YEAR * (100/step));
        case "last_month":
            return Math.round(STEPS_MONTH * (100/step));
        case "last_week":
            return Math.round(STEPS_WEEK * (100/step));
        case "last_day":
            return Math.round(STEPS_DAY * (100/step));
        default:
            return 0;
    }
}

// returns legends for charts from array of labels
function GetScalesFromLabels(y_labels) {
    let scales1 = {
        yAxes: []
    }

    if (!y_labels || y_labels.length == 0) {
        scales1.yAxes.push({
            id: 'axis0',
            scaleLabel: {
                display: false
            },
            type: 'linear'
        });
        return scales1;
    }

    scales1.yAxes.push({
        id: 'axis0',
        scaleLabel: {
            display: true,
            labelString: y_labels[0]
        },
        type: 'linear'
    });

    if (y_labels.length > 1) {
        let id_i;
        for (let i = 1; i < y_labels.length; i++) {
            id_i = 'axis' + i.toString();
            scales1.yAxes.push({
                id: id_i,
                position: 'right',
                scaleLabel: {
                    display: true,
                    labelString: y_labels[i]
                },
                type: 'linear'
            });
        }
    }
    return scales1;
}

// sets block_axis and time_axis for charts
function SetDataLabel(chart, rangeObj) {
    if (chart.data.labelset == "blocks") {
        chart.data.labels = rangeObj.blocks_axis;
        chart.data.hiddenlabels = rangeObj.time_axis;
    } else {
        chart.data.labels = rangeObj.time_axis;
        chart.data.hiddenlabels = rangeObj.blocks_axis;
    }
}

// CANVAS FRAME FUNCS

// sets chart scale (linear - logarithmic)
function SetScale(type_str, chart) {
    for (let i = 0; i < chart.options.scales.yAxes.length; i++) {
        chart.options.scales.yAxes[i].type = type_str;
        if (type_str == "logarithmic") {
            chart.options.scales.yAxes[i].ticks = {
                autoSkip: true,
                callback: function (value, index, values) {
                    // only first three ticks
                    if (
                        Number.isInteger(Math.log10(value)) ||
                        Number.isInteger(Math.log10(value/2)) ||
                        Number.isInteger(Math.log10(value/3))
                    ) {
                        return value;
                    }
                }
            };
        } else {
            chart.options.scales.yAxes[i].ticks = {
                autoSkip: true
            };
        }
    }
    chart.update();
}

// toggle X Axis between blocks and dates
function SetXAxis(axis, chart) {
    if (axis != chart.data.labelset) {
        let temp = chart.data.labels
        chart.data.labels = chart.data.hiddenlabels;
        chart.data.hiddenlabels = temp;
        chart.data.labelset = axis;
    }
    chart.update();
}

// handle range logic for canvas
function onRangeSelectChanged(sel_id) {
    const sel_el = document.getElementById(sel_id);
    const sel_val = sel_el.options[sel_el.selectedIndex].value;
    const div_id1 = sel_id + "_from";
    const div_id2 = sel_id + "_to";
    const div_id3 = sel_id + "_btn";
    const div_from = document.getElementById(div_id1);
    const div_to = document.getElementById(div_id2);
    const div_btn = document.getElementById(div_id3);
    if (sel_val == "custom") {
        // show hidden form for custom range
        div_from.parentNode.style.display = "block";
        div_to.parentNode.style.display = "block";
        div_btn.parentNode.style.display = "block";
        // init default _to value if not set
        if (div_to.value == "1") {
            div_to.value = LAST_BLOCK_NUM;
        }
    } else {
        // hide form for custom range
        div_from.parentNode.style.display = "none";
        div_to.parentNode.style.display = "none";
        div_btn.parentNode.style.display = "none";
        // call default func
        SetChartRange(sel_id, sel_val);
    }
}

function SetChartRange(canv_id, range) {
    let step = 100;
    if (canv_id == "canv_testing_01") {
      step = 1;
    }
    const total_len = Math.round((LAST_BLOCK_NUM + 1)/step);
    HideCanvas(canv_id);
    let val_from, val_to;

    if (range) {
        const timeRange = GetTimeRange(range, step);
        val_from = total_len - timeRange;
        val_to = total_len - 1;
        // timeRange = 0 is used for whole dataset
        if (timeRange == 0) {
            val_from = 0;
        }
    } else {
        // custom range
        val_from = document.getElementById(canv_id + "_from").value;
        val_to = document.getElementById(canv_id + "_to").value;
        val_from = Math.round(val_from/step)-1;
        val_to = Math.round(val_to/step)-1;
    }

    if (val_to >= total_len) {
        val_to = total_len - 1;
    }

    if (val_from >= val_to) {
        alert("Invalid range. Last block number must be greater than first block number (+ 100)");
        return;
    }

    // call appropriate function
    MapIdToSetChartRange(canv_id, val_from, val_to);
}

function ShowCanvas(canv_id) {
    document.getElementById(canv_id + "_spinner").style.display = "none";
    document.getElementById(canv_id + "_canvas_wrapper").style.display = "block";
}

function HideCanvas(canv_id) {
    document.getElementById(canv_id + "_spinner").style.display = "block";
    document.getElementById(canv_id + "_canvas_wrapper").style.display = "none";
}

function MinimizeChartLegend(chart, size, break_point) {
    if (!break_point) {
        break_point = 600;
    }
    if (size.width < break_point) {
        // show toggle-legend button
        document.getElementById(chart.canvas.id + "_legendbtn").style.display = "block";
        // hide legend and move it to bottom
        chart.options.legend.display = false;
        chart.options.legend.position = 'bottom';
        // tooltips only on intersect
        chart.options.tooltips.intersect = true;
        // remove points
        chart.data.datasets.forEach(ds => {ds.pointRadius = 0});
        chart.update();
    } else {
        // hide toggle-legend button
        document.getElementById(chart.canvas.id + "_legendbtn").style.display = "none";
        // restore position and show legend
        if (chart.options.legend.origPosition) {
            chart.options.legend.position = chart.options.legend.origPosition;
        }
        chart.options.legend.display = true;
        // tooltips always on hover
        chart.options.tooltips.intersect = false;
        // restore points
        chart.data.datasets.forEach(ds => {
            if (ds.needsRadius) {
                ds.pointRadius = ds.needsRadius;
            } else {
                ds.pointRadius = 1;
            }
        });
        chart.update();
    }
}

function ToggleChartLegend(chart) {
    chart.options.legend.display = !chart.options.legend.display;
    chart.update();
}

const verticalLinePlugin = {
   afterDatasetsDraw: function(chart) {
      if (chart.tooltip._active && chart.tooltip._active.length) {
         var activePoint = chart.tooltip._active[0],
             ctx = chart.ctx,
             y_axis = chart.scales.axis0,
             x = activePoint.tooltipPosition().x,
             topY = y_axis.top,
             bottomY = y_axis.bottom;
         // draw line
         ctx.save();
         ctx.beginPath();
         ctx.moveTo(x, topY);
         ctx.lineTo(x, bottomY);
         ctx.lineWidth = 2;
         ctx.strokeStyle = '#07C';
         ctx.stroke();
         ctx.restore();
      }
   }
}
