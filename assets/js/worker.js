var interval;

self.addEventListener('message', function (e) {
    if (e.data == 'start') {
        setTimeout(function () {
            self.postMessage('tick');
        }, 3000);
    }
}, false);