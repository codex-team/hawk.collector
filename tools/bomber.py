import requests
from datetime import datetime

data = '''{"token":"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9","sender":{"ip":"127.0.0.1"},"catcher_type":"errors/php","payload":{"title":"Field `blocks` is missing","timestamp":1545203808,"severity":16,"backtrace":[{"file":"/var/www/codex/vendor/codex-team/editor.js/EditorJS/EditorJS.php","line":77,"source code":[{"line number":76,"content":"         if (!isset($data['blocks'])) {"},{"line number":77,"content":"             throw new EditorJSException('Field `blocks` is missing');"},{"line number":78,"content":"         }"}]},{"file":"/var/www/codex/application/classes/Controller/Articles/Index.php","called line":"191","source code":[{"line number":"190","content":"     {"},{"line number":"191","content":"         $editor = new EditorJS($content, Model_Article::getEditorConfig());"},{"line number":"192","content":"         $blocks = $editor->getBlocks();"}]}],"get":{},"post":{"text":"Hello, World!","is_published":false},"headers":{},"source release":""}}'''


import random
import asyncio
from aiohttp import ClientSession

async def fetch(url, session):
    async with session.post(url, data=data) as response:
        #delay = response.headers.get("DELAY")
        #date = response.headers.get("DATE")
        #print("{}:{} with delay {}".format(date, response.url, delay))
        return await response.read()


async def bound_fetch(sem, url, session):
    # Getter function with semaphore.
    async with sem:
        await fetch(url, session)


async def run(r):
    url = "http://localhost:3000/catcher"
    tasks = []
    # create instance of Semaphore
    sem = asyncio.Semaphore(100)

    # Create client session that will ensure we dont open new connection
    # per each request.
    async with ClientSession() as session:
        for i in range(r):
            # pass Semaphore and session to every GET request
            task = asyncio.ensure_future(bound_fetch(sem, url, session))
            tasks.append(task)

        responses = asyncio.gather(*tasks)
        await responses

number = 100
loop = asyncio.get_event_loop()
t = datetime.now()
future = asyncio.ensure_future(run(number))
loop.run_until_complete(future)
print(datetime.now() - t)