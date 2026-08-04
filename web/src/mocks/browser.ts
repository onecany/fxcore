// MSW 版本化入口（API设计.md 7.2：Mock 代码按版本存放于 handlers/v1/）。
import { setupWorker } from 'msw/browser';
import { authHandlers } from './handlers/v1/auth';
import { traderHandlers } from './handlers/v1/traders';

export const worker = setupWorker(...authHandlers, ...traderHandlers);
