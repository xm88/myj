# 迁移参考

这里保留迁移前的 TypeScript 实现，供协议、数值和行为回归对照。
默认 `npm run dev`、`npm start`、`npm test` 均执行 `server/*.go`，不会启动这里的服务端。
运行时配置源为根目录 `config/*.json`。

`smoke.ts` 与 `loop.ts` 是客户端回归脚本，可对 Go 服务执行：

```powershell
$env:FLOAT_URL = 'http://127.0.0.1:8787'
npx tsx server/src/smoke.ts
npx tsx server/src/loop.ts
```
