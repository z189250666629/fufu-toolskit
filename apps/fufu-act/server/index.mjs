import express from 'express';
import cors from 'cors';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import { createDb } from './db.mjs';
import { createRouter } from './routes.mjs';
import { startCreditWorker } from './credit-worker.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PORT = process.env.SLOT_PORT || 18820;

const app = express();
app.use(cors());
app.use(express.json());

// 静态前端
app.use(express.static(join(__dirname, '..', 'public')));

// 数据库
const db = createDb(join(__dirname, '..', 'data', 'slot.db'));

// API 路由
app.use('/api', createRouter(db));

// 异步充值队列
startCreditWorker(db);

const server = app.listen(PORT, '0.0.0.0', () => {
  console.log(`🎰 Slot machine server running on port ${PORT}`);
});

// Keep process alive
server.on('error', (err) => {
  console.error('Server error:', err);
  process.exit(1);
});
