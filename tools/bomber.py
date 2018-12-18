import requests
from datetime import datetime

data = '{"_id":"5c10c72e62200104e35fb456","type":0,"uid":"7dc374a1-5dde-465c-b59a-eeb82b414c5f","jwt":true,"payload":"Adipisicing amet nulla mollit consectetur in nisi adipisicing pariatur commodo. Sint pariatur ut ullamco laboris excepteur proident do. Exercitation irure in ullamco anim consequat esse laborum labore aliqua duis ea fugiat tempor officia. Irure non anim nostrud ut ad aute qui in minim exercitation amet esse. Cupidatat ad occaecat Lorem aliquip tempor elit non exercitation ut cupidatat irure et duis. Aliqua do non non anim ut. Ullamco deserunt eu dolore dolor pariatur laboris cillum ea ullamco ut.rn"}'


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
    url = "http://localhost:8080/catcher"
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

number = 25000
loop = asyncio.get_event_loop()
t = datetime.now()
future = asyncio.ensure_future(run(number))
loop.run_until_complete(future)
print(datetime.now() - t)