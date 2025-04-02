var chartDom = document.getElementById('main');
var myChart = echarts.init(chartDom);
var option;

let nbBlocks = 2
let defaultFee = 0.01
let feesAIMD = [];
let fees_eip1559 = [];
let defaultBlockSize = 0;
let blockSize = []
let maxBlockSize = 30000000
let currentLearningRate = 0

for (var i = 0; i < nbBlocks; i++) {
    feesAIMD.push({
        value: [i, defaultFee]
    });
    fees_eip1559.push({
        value: [i, defaultFee]
    });
    blockSize.push({
        value: [i, defaultBlockSize]
    });
}
option = {
    title: {
        text: 'Feemarket throttling'
    },
    tooltip: {
        trigger: 'axis',
        formatter: function(params) {
            return 'height=' + params[0].data.value[0] +
                ' Block Size=' + params[2].data.value[1] +
                ' Gas Price AIMD=' + params[0].data.value[1] +
                ' Gas Price EIP1559=' + params[1].data.value[1]
        },
        axisPointer: {
            animation: false
        }
    },
    xAxis: {
        name: 'Block height',
    },
    yAxis: [{
        name: 'Gas Price',
        type: 'value',
        boundaryGap: [0, '25%'],
    }, {
        name: 'currentBlockSize',
        min: 0,
        max: maxBlockSize,
    }, ],

};

function reset() {
    feesAIMD = [];
    fees_eip1559 = []
    blockSize = []
    nbBlocks = 0;
    document.getElementById('baseGasPrice').value = defaultFee;
    document.getElementById('currentBlockSize').value = defaultBlockSize;
    document.getElementById('targetBlockSize').value = maxBlockSize / 2;
    document.getElementById('maxBlockSize').value = maxBlockSize;
    document.getElementById('alpha').value = 0.025;
    document.getElementById('beta').value = 0.25;
    document.getElementById('window').value = 8;
    document.getElementById('gamma').value = 0.25;
    document.getElementById('maxLearningRate').value = 0.5;
    document.getElementById('minLearningRate').value = 0.01;
    document.getElementById('delta').value = 0.0;
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

function computeFee(n) {
    let lastFeeAIMD = defaultFee
    let lastFeeEIP1559 = defaultFee
    if (feesAIMD.length > 0) {
        lastFeeAIMD = feesAIMD[feesAIMD.length - 1].value[1]
        lastFeeEIP1559 = fees_eip1559[fees_eip1559.length - 1].value[1]
    }
    let baseGasPrice = parseFloat(document.getElementById('baseGasPrice').value);
    let alpha = parseFloat(document.getElementById('alpha').value);
    let beta = parseFloat(document.getElementById('beta').value);
    let delta = parseFloat(document.getElementById('delta').value);
    let window = parseFloat(document.getElementById('window').value);
    let gamma = parseFloat(document.getElementById('gamma').value);
    let currentBlockSize = parseFloat(document.getElementById('currentBlockSize').value);
    let targetBlockSize = parseFloat(document.getElementById('targetBlockSize').value);
    let maxBlockSize = parseFloat(document.getElementById('maxBlockSize').value);
    let maxLearningRate = parseFloat(document.getElementById('maxLearningRate').value);
    let minLearningRate = parseFloat(document.getElementById('minLearningRate').value);


    let beginningOfWindow = Math.max(0, blockSize.length - 1 - window);
    console.log(`beginningOfWindow= ${beginningOfWindow}`)

    let blockConsumption = 0;
    for (var i = beginningOfWindow; i < Math.min(beginningOfWindow + window, blockSize.length - 1); i++) {
        blockConsumption += blockSize[i].value[1];
        console.log(`blockSize[${i}]= ${blockSize[i].value[1]}`)
    }
    console.log(`window = ${window}`)
    console.log(`maxBlockSize = ${maxBlockSize}`)
    blockConsumption /= window;
    blockConsumption /= maxBlockSize;
    console.log(`blockConsumption= ${blockConsumption}`)

    if (currentLearningRate == 0) {
        currentLearningRate = minLearningRate;
    }
    var newLearningRate = Math.max(minLearningRate, beta * currentLearningRate);
    if (blockConsumption < gamma ||
        blockConsumption > 1 - gamma) {
        console.log(`blockConsumption < gamma => True`)
        newLearningRate = Math.min(maxLearningRate, alpha + currentLearningRate);
    }
    console.log(`newLearningRate = ${newLearningRate}`)
    currentLearningRate = newLearningRate;
    console.log(`currentBlockSize = ${currentBlockSize}`)
    console.log(`targetBlockSize = ${targetBlockSize}`)

    var netGasDelta = 0;
    for (var i = beginningOfWindow; i < Math.min(beginningOfWindow + window, blockSize.length - 1); i++) {
        let utilization = blockSize[i].value[1];
        netGasDelta += utilization - targetBlockSize;
    }
    console.log(`netGasDelta = ${netGasDelta}`)
    var AIMD_F = lastFeeAIMD * (1 + (newLearningRate * (currentBlockSize - targetBlockSize)) / targetBlockSize) + delta * netGasDelta;

    var eip1559LearningRate = 0.125
    var eip1559_F = lastFeeEIP1559 * (1 + (eip1559LearningRate * (currentBlockSize - targetBlockSize)) / targetBlockSize);
    console.log(`lastFee AIMD = ${lastFeeAIMD}`)
    console.log(`lastFee EIP1559 = ${lastFeeEIP1559}`)
    console.log(`currentFeeAIMD = ${AIMD_F}`)
    console.log(`currentFeeEIP1559 = ${eip1559_F}`)

    AIMD_F = Math.max(AIMD_F, baseGasPrice)
    eip1559_F = Math.max(eip1559_F, baseGasPrice)

    return [AIMD_F, eip1559_F];
}

setInterval(function() {
    if (paused) {
        return
    }
    nbBlocks++
    n = parseInt(document.getElementById('currentBlockSize').value);
    // deposits.shift();
    var computedFees = computeFee(n);
    feesAIMD.push({
        value: [nbBlocks, computedFees[0]]
    });
    fees_eip1559.push({
        value: [nbBlocks, computedFees[1]]
    });
    // numProposals.shift();
    blockSize.push({
        value: [nbBlocks, n]
    });
    myChart.setOption({
        series: [{
            name: 'Fees AIMD',
            type: 'line',
            data: feesAIMD,
            symbolSize: 3,
        }, {
            name: 'Fees EIP1559',
            type: 'line',
            data: fees_eip1559,
            symbolSize: 3,
        }, {
            name: 'Block Size',
            type: 'line',
            data: blockSize,
            yAxisIndex: 1,
            symbolSize: 3,
        }],
        legend: {
            data: ['Block Size', 'Fees AIMD', 'Fees EIP1559'],
        },
    });
}, 1000);

option && myChart.setOption(option);
