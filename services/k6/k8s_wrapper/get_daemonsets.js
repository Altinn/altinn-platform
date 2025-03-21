import { check } from 'k6';
import http from 'k6/http';
import { sleep } from 'k6';
import exec from 'k6/execution';

export function setup() {
    console.log(JSON.stringify(exec.test.options, null, "\t"))
}

export default function () {
    const res = http.get(`${__ENV.BASE_URL}/kuberneteswrapper/api/v1/DaemonSets`)
    check(res, {
        'is status 200': (r) => r.status === 200,
    });
    sleep(1);
}
