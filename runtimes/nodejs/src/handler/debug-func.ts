/*
 * @Author: Maslow<wangfugen@126.com>
 * @Date: 2021-07-30 10:30:29
 * @LastEditTime: 2022-02-03 00:59:03
 * @Description:
 */

import { Response } from 'express'
import { FunctionContext } from '../support/function-engine'
import { logger } from '../support/logger'
import { CloudFunction } from '../support/function-engine'
import { IRequest } from '../support/types'
import { parseToken } from '../support/token'

/**
 * Handler of debugging cloud function
 */
export async function handleDebugFunction(req: IRequest, res: Response) {
  // verify the debug token
  const token = req.get('x-laf-debug-token')
  if (!token) {
    return res.status(400).send('x-laf-debug-token is required')
  }
  const auth = parseToken(token) || null
  if (auth?.type !== 'debug') {
    return res.status(403).send('permission denied: invalid debug token')
  }

  const requestId = req['requestId']
  const func_name = req.params?.name
  const func_data = req.body?.func
  const param = req.body?.param

  if (!func_data) {
    return res.send({ code: 1, error: 'function data not found', requestId })
  }

  const func = new CloudFunction(func_data)

  try {
    // execute the func
    const ctx: FunctionContext = {
      query: req.query,
      files: req.files as any,
      body: param,
      headers: req.headers,
      method: req.method,
      auth: req.user,
      user: req.user,
      requestId,
      response: res,
      __function_name: func.name,
    }
    const result = await func.invoke(ctx)

    if (result.error) {
      logger.error(requestId, `debug function ${func_name} error: `, result)
      return res.send({
        error: 'invoke function got error: ' + result.error.toString(),
        time_usage: result.time_usage,
        requestId,
      })
    }

    logger.trace(requestId, `invoke ${func_name} invoke success: `, result)

    if (res.writableEnded === false) {
      let data = result.data
      if (typeof result.data === 'number') {
        data = Number(result.data).toString()
      }
      return res.send(data)
    }
  } catch (error) {
    logger.error(requestId, 'failed to invoke error', error)
    return res.status(500).send('Internal Server Error')
  }
}
