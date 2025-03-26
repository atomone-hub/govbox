var chartDom = document.getElementById('main');
var myChart = echarts.init(chartDom);
var option;

let nbBlocks = 0
let baseDeposit = parseFloat(document.getElementById('baseDeposit').value);
let deposits = [];
let numProposals = []
let maxDeposit = baseDeposit
for (var i = 0; i < nbBlocks; i++) {
    deposits.push({
        value: [i, baseDeposit]
    });
    numProposals.push({
        value: [i, 0]
    });
}
option = {
    title: {
        text: 'Deposit throttling'
    },
    tooltip: {
        trigger: 'axis',
        formatter: function(params) {
            return 'height=' + params[0].data.value[0] +
                ' n=' + params[1].data.value[1] +
                ' D=' + params[0].data.value[1]
        },
        axisPointer: {
            animation: false
        }
    },
    xAxis: {
        name: 'Block height',
    },
    yAxis: [{
        name: 'Deposit',
        type: 'value',
        boundaryGap: [0, '25%'],
    }, {
        name: 'Num proposals',
        min: 0,
        max: 8,
    }, ],
    legend: {
        data: ['Num proposals', 'Deposit'],
    }
};

function reset() {
    deposits = [];
    numProposals = [];
    nbBlocks = 0;
    lastn = 0
    document.getElementById('numProposals').value = 0;
}

let paused = false

function pauseResume() {
    if (paused) {
        paused = false
        document.getElementById('pauseResume').innerHTML = 'Pause'
    } else {
        document.getElementById('pauseResume').innerHTML = 'Resume'
        paused = true
    }
}

let lastn = 0

function computeDeposit(n) {
    let baseDeposit = parseFloat(document.getElementById('baseDeposit').value);
    let lastDeposit = baseDeposit;
    if (deposits.length > 0) {
        lastDeposit = deposits[deposits.length - 1].value[1];
    }
    let N = parseFloat(document.getElementById('N').value);
    let percChange = 1;
    console.log(`N=${N} n=${n} lastn=${lastn}`);
    let D = 0
    if (n < N) {
        let alpha = parseFloat(document.getElementById('alphaDown').value);
        let k = parseFloat(document.getElementById('k').value);
        let v = (Math.abs(n - N)) ** (1 / k);
        console.log(`k=${k} v=${v}`);
        percChange = 1 + alpha * v;
        D = maxDeposit - (maxDeposit - lastDeposit) * percChange;
    } else if (n >= N) {
        // increase only if there is new proposal(s) added since last computation
        if (n > lastn) {
            let alpha = parseFloat(document.getElementById('alphaUp').value);
            // if more than one proposal has been added in a single tick, 
            // handle that by powering the percentage change, but any proposals
            // added before N is reached should be ignored.
            percChange = (1 + alpha) ** (n - Math.max(lastn, N - 1));
        }
        D = lastDeposit * percChange;
        maxDeposit = D
    }
    lastn = n;

    if (D < baseDeposit) {
        D = baseDeposit;
    }
    console.log(`baseDeposit=${baseDeposit} lastDeposit=${lastDeposit} percChange=${percChange} maxDeposit=${maxDeposit}`);
    console.log(`----> D=${D}`);
    console.log('***********************************');
    return D;
}

function pushBlock(n) {
    nbBlocks++
    // deposits.shift();
    deposits.push({
        value: [nbBlocks, computeDeposit(n)]
    });
    // numProposals.shift();
    numProposals.push({
        value: [nbBlocks, n]
    })
}

setInterval(function() {
    if (paused) {
        return
    }
    blockWindow = parseInt(document.getElementById('blockWindow').value);
    n = parseInt(document.getElementById('numProposals').value);
    for (let i = 0; i < blockWindow; i++) {
        pushBlock(n)
    }
    myChart.setOption({
        series: [{
            name: 'Deposit',
            type: 'line',
            data: deposits,
            symbolSize: 3,
        }, {
            name: 'Num proposals',
            type: 'line',
            data: numProposals,
            yAxisIndex: 1,
            symbolSize: 3,
        }]
    });
}, 1000);

option && myChart.setOption(option);
