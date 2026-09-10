// Isolated layout-picker test. Node 22+ and CHROME (Chrome or Edge) are required.
// No SSH connections or user profiles are opened.
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import http from 'node:http'
import { spawn } from 'node:child_process'
import { setTimeout as delay } from 'node:timers/promises'

const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'ssh-grid-ui-'))
const script = await fs.readFile('web/js/host-batch.js', 'utf8')
const styles = await Promise.all(['style', 'host-control', 'host-order', 'dialog', 'modals', 'theme'].map(name => fs.readFile(`web/css/${name}.css`, 'utf8')))
const server = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'text/html; charset=utf-8')
    res.end(`<html><head><style>${styles.join('\n')}</style></head><body>
        <div id="hosts-data-container"><button onclick="hostBatch.groups()">Groups</button></div>
        <span id="host-selection-count"></span><button id="host-batch-open"></button>
        <button id="host-batch-save"></button><button id="host-batch-delete"></button>
    </body></html>`)
})
let browser, socket, call
try {
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
    browser = spawn(process.env.CHROME || 'google-chrome', ['--headless=new', '--no-proxy-server', '--remote-debugging-port=0', `--user-data-dir=${temporary}`, 'about:blank'], { stdio: 'ignore', windowsHide: true })
    browser.on('error', error => console.error(error.message))
    let port
    for (let i = 0; !port && i < 100; i++) {
        try { port = (await fs.readFile(path.join(temporary, 'DevToolsActivePort'), 'utf8')).split('\n')[0] }
        catch { await delay(100) }
    }
    assert.ok(port, 'Browser did not start; set CHROME to its executable path')
    const pages = await (await fetch(`http://127.0.0.1:${port}/json`)).json()
    socket = new WebSocket(pages.find(page => page.type === 'page').webSocketDebuggerUrl)
    await new Promise(resolve => socket.addEventListener('open', resolve, { once: true }))
    let sequence = 0
    const pending = new Map()
    socket.addEventListener('message', event => {
        const data = JSON.parse(event.data)
        if (data.id) { pending.get(data.id)?.(data); pending.delete(data.id) }
    })
    call = async (method, params = {}) => {
        const id = ++sequence
        let timer
        try {
            const response = await Promise.race([
                new Promise(resolve => { pending.set(id, resolve); socket.send(JSON.stringify({ id, method, params })) }),
                new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(method + ' timed out')), 10000) }),
            ])
            assert.ok(!response.error, JSON.stringify(response.error))
            return response.result
        } finally { clearTimeout(timer); pending.delete(id) }
    }
    const evaluate = async expression => {
        const response = await call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
        assert.ok(!response.exceptionDetails, JSON.stringify(response.exceptionDetails))
        return response.result.value
    }
    await call('Page.navigate', { url: `http://127.0.0.1:${server.address().port}` })
    for (let i = 0; i < 100 && !await evaluate(`!!document.querySelector('#hosts-data-container')`); i++) await delay(50)
    await evaluate(`
        var hostsFile='fixture', hostsData=[{hosts:['a','b','c'].map((id,i)=>({'unique-id':id,name:'Host '+(i+1)}))}];
        var appDialogs={alert:async message=>{throw new Error(message)}};
        var saved=[{id:'one',name:'Test',layout:'grid',columns:2,'grid-fill':'vertical','host-ids':['a','b','c']}], requests=[], rejectReorder=false;
        window.fetch=async(url,options={})=>{
            let data={};
            if(url.startsWith('/connection-groups')) {
                if(options.method==='PUT') saved=[JSON.parse(options.body)];
                if(options.method==='PATCH') {
                    if(rejectReorder) return {ok:false,text:async()=> 'Test reorder failure'};
                    saved=JSON.parse(options.body)['group-ids'].map(id=>saved.find(group=>group.id===id));
                }
                data=url.includes('settings=panel')?{'panel-pinned':false}:saved;
            } else if(url==='/session/batch') {requests.push(JSON.parse(options.body));data={opened:3};}
            return {ok:true,json:async()=>data};
        };
        ${script}
    `)
    for (const [width, height] of [[1100, 800], [390, 844]]) {
        await call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
        await delay(100)
        for (const mode of ['group', 'modal']) {
            await evaluate(mode === 'group' ? `hostBatch.groups()` : `hostBatch.open(['a','b','c'],'grid:2:vertical')`)
            const root = mode === 'group' ? '#connection-groups-panel' : '#host-batch-dialog'
            if (mode === 'group') {
                assert.equal(await evaluate(`(()=>{const card=document.querySelector('.connection-group-list > section'), split=card.querySelector('.grid-picker-toggle').getBoundingClientRect(), connect=card.querySelector('[data-action="connect"]').getBoundingClientRect(), actions=card.querySelector('.connection-group-actions').getBoundingClientRect();return Math.abs(split.left-connect.left)<1&&Math.abs(split.right-actions.right)<1})()`), true, 'Connect row aligns with Split control')
            }
            await evaluate(`document.querySelector('${root} .grid-picker-toggle').click()`)
            for (const [fill, preview] of [['vertical-left',[['1'],['2','3']]],['vertical',[['1','3'],['2']]],['horizontal',[['1','2'],['3']]],['vertical-left',[['1'],['2','3']]]]) {
                await evaluate(`(async()=>{const s=document.querySelector('.grid-fill-select');s.value=${JSON.stringify(fill)};await s.onchange()})()`)
                assert.deepEqual(await evaluate(`[...document.querySelector('.grid-arrangement').children].map(strip=>[...strip.children].map(cell=>cell.textContent))`), preview)
            }
            assert.deepEqual(await evaluate(`[...document.querySelector('.grid-arrangement').children].map(strip=>[...strip.children].map(cell=>cell.textContent))`), [['1'], ['2', '3']])
            assert.equal(await evaluate(`document.querySelector('${root} select').value`), 'grid:2:vertical-left')
            assert.equal(await evaluate(`(()=>{const r=document.querySelector('.layout-picker').getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight})()`), true, 'Picker fits viewport')
            const shot = await call('Page.captureScreenshot', { format: 'png' })
            await fs.writeFile(path.join(temporary, `${mode}-${width}.png`), Buffer.from(shot.data, 'base64'))
            await evaluate(`document.querySelector('${root} .grid-picker-toggle').click()`)
            await evaluate(mode === 'group' ? `document.querySelector('${root} [data-action="connect"]').click()` : `document.querySelector('${root} [data-connect]').click()`)
            for (let i=0;i<100 && !await evaluate('requests.length');i++) await delay(20)
            assert.equal(await evaluate(`requests.pop()['grid-fill']`), 'vertical-left')
            await delay(50)
            if (mode === 'modal') await evaluate(`document.querySelector('#host-batch-dialog').close()`)
        }
    }
    assert.equal(await evaluate(`saved[0]['grid-fill']`), 'vertical-left')
    await evaluate(`saved=['one','two','three'].map((id,i)=>({...saved[0],id,name:'Group '+(i+1)}));hostBatch.groups()`)
    const order = () => evaluate(`[...document.querySelectorAll('.connection-group-list > section')].map(card=>card.dataset.groupId)`)
    const settled = async () => {
        for (let i=0;i<100;i++) {
            if (await evaluate(`!document.querySelector('.connection-group-name').disabled`)) return
            await delay(20)
        }
        throw new Error('Group save did not finish')
    }
    await evaluate(`document.querySelector('[data-group-id="two"] .connection-group-name').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowUp',altKey:true,bubbles:true}))`)
    await settled()
    assert.deepEqual(await order(), ['two','one','three'])
    assert.equal(await evaluate(`document.activeElement.closest('[data-group-id]').dataset.groupId`), 'two')
    await evaluate(`(()=>{
        const source=document.querySelector('[data-group-id="three"] .connection-group-name'), target=document.querySelector('[data-group-id="two"]'), dataTransfer=new DataTransfer();
        source.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer}));
        target.dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer,clientY:target.getBoundingClientRect().top+1}));
    })()`)
    await settled()
    assert.deepEqual(await order(), ['three','two','one'])
    await evaluate(`document.querySelector('[data-group-id="three"] [data-action="menu"]').click()`)
    assert.equal(await evaluate(`[...document.querySelectorAll('.connection-group-menu button')].find(b=>b.textContent==='Move up').disabled`), true)
    await evaluate(`[...document.querySelectorAll('.connection-group-menu button')].find(b=>b.textContent==='Move down').click()`)
    await settled()
    assert.deepEqual(await order(), ['two','three','one'])
    await evaluate(`hostBatch.groups()`)
    assert.deepEqual(await order(), ['two','three','one'])
    await evaluate(`rejectReorder=true;document.querySelector('[data-group-id="one"] .connection-group-name').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowUp',altKey:true,bubbles:true}))`)
    await settled()
    assert.deepEqual(await order(), ['two','three','one'])
    assert.equal(await evaluate(`document.querySelector('.group-result').textContent`), 'Test reorder failure')
    assert.deepEqual(await evaluate(`saved.map(g=>g['host-ids'])`), [['a','b','c'],['a','b','c'],['a','b','c']])
    console.log('PASS: group keyboard/drag/menu ordering, restored focus, reload, failed-save preservation, unchanged member order')
    const broadcast = await fs.readFile('web/js/broadcast.js', 'utf8')
    await evaluate(`
        document.body.insertAdjacentHTML('beforeend','<button id="broadcast-tool"><span id="broadcast-indicator"></span></button>');
        var broadcastState={connections:['a','b','c'].map(id=>({id,label:id})),source:'',targets:[],enabled:false}, broadcastReads=0, broadcastWrites=0;
        var previousFetch=window.fetch;
        window.fetch=async(url,options={})=>{
            if(url==='/session/broadcast') {
                if(options.method==='POST') broadcastWrites++; else broadcastReads++;
                return {ok:true,json:async()=>structuredClone(broadcastState)};
            }
            return previousFetch(url,options);
        };
        ${broadcast}
        inputBroadcast.open();
    `)
    const broadcastSelection = () => evaluate(`({source:document.querySelector('#broadcast-dialog input[type="radio"]:checked')?.value||'',targets:[...document.querySelectorAll('#broadcast-dialog tbody input[type="checkbox"]:checked')].map(input=>input.value)})`)
    const waitBroadcast = async before => {
        for(let i=0;i<100;i++) {
            if(await evaluate(`broadcastReads>${before} && !document.querySelector('#broadcast-dialog [data-start]').disabled`)) return
            await delay(20)
        }
        throw new Error('Broadcast defaults did not load')
    }
    await waitBroadcast(0)
    assert.deepEqual(await broadcastSelection(), {source:'a',targets:['b','c']})
    await evaluate(`const receiver=document.querySelector('#broadcast-dialog tbody input[type="checkbox"][value="b"]');receiver.checked=false;receiver.dispatchEvent(new Event('change',{bubbles:true}));broadcastState.connections.push({id:'d',label:'d'})`)
    await delay(1200)
    assert.deepEqual(await broadcastSelection(), {source:'a',targets:['c']}, 'Polling preserves edits and does not select new receivers')
    let reads = await evaluate('broadcastReads')
    await evaluate(`document.querySelector('#broadcast-dialog').close();inputBroadcast.open()`)
    await waitBroadcast(reads)
    assert.deepEqual(await broadcastSelection(), {source:'a',targets:['b','c','d']})
    for (const state of [
        {connections:[{id:'a',label:'a'},{id:'b',label:'b'},{id:'c',label:'c'}],source:'b',targets:['c'],enabled:true},
        {connections:[{id:'a',label:'a'}],source:'',targets:[],enabled:false},
        {connections:[],source:'',targets:[],enabled:false},
    ]) {
        reads=await evaluate('broadcastReads')
        await evaluate(`broadcastState=${JSON.stringify(state)};document.querySelector('#broadcast-dialog').close();inputBroadcast.open()`)
        for(let i=0;i<100 && !await evaluate(`broadcastReads>${reads}`);i++) await delay(20)
        await delay(30)
        assert.deepEqual(await broadcastSelection(), state.enabled ? {source:'b',targets:['c']} : {source:state.connections[0]?.id||'',targets:[]})
        assert.equal(await evaluate(`document.querySelector('#broadcast-dialog [data-start]').disabled`),true)
    }
    assert.equal(await evaluate('broadcastWrites'),0,'Opening never starts or reconfigures broadcast')
    console.log('PASS: broadcast opening defaults, polling preservation, reopen, active broadcast, single/empty lists; no automatic POST')
    console.log('PASS: group and modal previews, persistence, batch requests, desktop/mobile bounds. Screenshots: ' + temporary)
} finally {
    if (call) { try { await call('Browser.close') } catch {} }
    socket?.close()
    if (browser && browser.exitCode === null) {
        await Promise.race([new Promise(resolve=>browser.once('exit',resolve)),delay(2000)])
        if (browser.exitCode === null) browser.kill()
    }
    await new Promise(resolve=>server.close(resolve))
}
