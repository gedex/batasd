const fs = require("node:fs");

const name = fs.readFileSync(0, "utf8").trim();
console.log(`hello from node: ${name}`);
