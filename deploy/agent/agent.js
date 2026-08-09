"use strict";
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __esm = (fn, res, err) => function __init() {
  if (err) {throw err[0];}
  try {
    return fn && (res = (0, fn[__getOwnPropNames(fn)[0]])(fn = 0)), res;
  } catch (e) {
    throw err = [e], e;
  }
};
var __commonJS = (cb, mod) => function __require() {
  try {
    return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
  } catch (e) {
    throw mod = 0, e;
  }
};
var __export = (target, all) => {
  for (var name in all)
    {__defProp(target, name, { get: all[name], enumerable: true });}
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      {if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });}
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/constants.js
var require_constants = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/constants.js"(exports2, module2) {
    "use strict";
    var BINARY_TYPES = ["nodebuffer", "arraybuffer", "fragments"];
    var hasBlob = typeof Blob !== "undefined";
    if (hasBlob) {BINARY_TYPES.push("blob");}
    module2.exports = {
      BINARY_TYPES,
      CLOSE_TIMEOUT: 3e4,
      EMPTY_BUFFER: Buffer.alloc(0),
      GUID: "258EAFA5-E914-47DA-95CA-C5AB0DC85B11",
      hasBlob,
      kForOnEventAttribute: /* @__PURE__ */ Symbol("kIsForOnEventAttribute"),
      kListener: /* @__PURE__ */ Symbol("kListener"),
      kStatusCode: /* @__PURE__ */ Symbol("status-code"),
      kWebSocket: /* @__PURE__ */ Symbol("websocket"),
      NOOP: () => {
      }
    };
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/buffer-util.js
var require_buffer_util = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/buffer-util.js"(exports2, module2) {
    "use strict";
    var { EMPTY_BUFFER } = require_constants();
    var FastBuffer = Buffer[Symbol.species];
    function concat(list, totalLength) {
      if (list.length === 0) {return EMPTY_BUFFER;}
      if (list.length === 1) {return list[0];}
      const target = Buffer.allocUnsafe(totalLength);
      let offset = 0;
      for (let i = 0; i < list.length; i++) {
        const buf = list[i];
        target.set(buf, offset);
        offset += buf.length;
      }
      if (offset < totalLength) {
        return new FastBuffer(target.buffer, target.byteOffset, offset);
      }
      return target;
    }
    function _mask(source, mask, output, offset, length) {
      for (let i = 0; i < length; i++) {
        output[offset + i] = source[i] ^ mask[i & 3];
      }
    }
    function _unmask(buffer, mask) {
      for (let i = 0; i < buffer.length; i++) {
        buffer[i] ^= mask[i & 3];
      }
    }
    function toArrayBuffer(buf) {
      if (buf.length === buf.buffer.byteLength) {
        return buf.buffer;
      }
      return buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.length);
    }
    function toBuffer(data) {
      toBuffer.readOnly = true;
      if (Buffer.isBuffer(data)) {return data;}
      let buf;
      if (data instanceof ArrayBuffer) {
        buf = new FastBuffer(data);
      } else if (ArrayBuffer.isView(data)) {
        buf = new FastBuffer(data.buffer, data.byteOffset, data.byteLength);
      } else {
        buf = Buffer.from(data);
        toBuffer.readOnly = false;
      }
      return buf;
    }
    module2.exports = {
      concat,
      mask: _mask,
      toArrayBuffer,
      toBuffer,
      unmask: _unmask
    };
    if (!process.env.WS_NO_BUFFER_UTIL) {
      try {
        const bufferUtil = require("bufferutil");
        module2.exports.mask = function(source, mask, output, offset, length) {
          if (length < 48) {_mask(source, mask, output, offset, length);}
          else {bufferUtil.mask(source, mask, output, offset, length);}
        };
        module2.exports.unmask = function(buffer, mask) {
          if (buffer.length < 32) {_unmask(buffer, mask);}
          else {bufferUtil.unmask(buffer, mask);}
        };
      } catch (e) {
      }
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/limiter.js
var require_limiter = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/limiter.js"(exports2, module2) {
    "use strict";
    var kDone = /* @__PURE__ */ Symbol("kDone");
    var kRun = /* @__PURE__ */ Symbol("kRun");
    var Limiter = class {
      /**
       * Creates a new `Limiter`.
       *
       * @param {Number} [concurrency=Infinity] The maximum number of jobs allowed
       *     to run concurrently
       */
      constructor(concurrency) {
        this[kDone] = () => {
          this.pending--;
          this[kRun]();
        };
        this.concurrency = concurrency || Infinity;
        this.jobs = [];
        this.pending = 0;
      }
      /**
       * Adds a job to the queue.
       *
       * @param {Function} job The job to run
       * @public
       */
      add(job) {
        this.jobs.push(job);
        this[kRun]();
      }
      /**
       * Removes a job from the queue and runs it if possible.
       *
       * @private
       */
      [kRun]() {
        if (this.pending === this.concurrency) {return;}
        if (this.jobs.length) {
          const job = this.jobs.shift();
          this.pending++;
          job(this[kDone]);
        }
      }
    };
    module2.exports = Limiter;
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/permessage-deflate.js
var require_permessage_deflate = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/permessage-deflate.js"(exports2, module2) {
    "use strict";
    var zlib = require("node:zlib");
    var bufferUtil = require_buffer_util();
    var Limiter = require_limiter();
    var { kStatusCode } = require_constants();
    var FastBuffer = Buffer[Symbol.species];
    var TRAILER = Buffer.from([0, 0, 255, 255]);
    var kPerMessageDeflate = /* @__PURE__ */ Symbol("permessage-deflate");
    var kTotalLength = /* @__PURE__ */ Symbol("total-length");
    var kCallback = /* @__PURE__ */ Symbol("callback");
    var kBuffers = /* @__PURE__ */ Symbol("buffers");
    var kError = /* @__PURE__ */ Symbol("error");
    var zlibLimiter;
    var PerMessageDeflate2 = class {
      /**
       * Creates a PerMessageDeflate instance.
       *
       * @param {Object} [options] Configuration options
       * @param {(Boolean|Number)} [options.clientMaxWindowBits] Advertise support
       *     for, or request, a custom client window size
       * @param {Boolean} [options.clientNoContextTakeover=false] Advertise/
       *     acknowledge disabling of client context takeover
       * @param {Number} [options.concurrencyLimit=10] The number of concurrent
       *     calls to zlib
       * @param {Boolean} [options.isServer=false] Create the instance in either
       *     server or client mode
       * @param {Number} [options.maxPayload=0] The maximum allowed message length
       * @param {(Boolean|Number)} [options.serverMaxWindowBits] Request/confirm the
       *     use of a custom server window size
       * @param {Boolean} [options.serverNoContextTakeover=false] Request/accept
       *     disabling of server context takeover
       * @param {Number} [options.threshold=1024] Size (in bytes) below which
       *     messages should not be compressed if context takeover is disabled
       * @param {Object} [options.zlibDeflateOptions] Options to pass to zlib on
       *     deflate
       * @param {Object} [options.zlibInflateOptions] Options to pass to zlib on
       *     inflate
       */
      constructor(options) {
        this._options = options || {};
        this._threshold = this._options.threshold !== void 0 ? this._options.threshold : 1024;
        this._maxPayload = this._options.maxPayload | 0;
        this._isServer = !!this._options.isServer;
        this._deflate = null;
        this._inflate = null;
        this.params = null;
        if (!zlibLimiter) {
          const concurrency = this._options.concurrencyLimit !== void 0 ? this._options.concurrencyLimit : 10;
          zlibLimiter = new Limiter(concurrency);
        }
      }
      /**
       * @type {String}
       */
      static get extensionName() {
        return "permessage-deflate";
      }
      /**
       * Create an extension negotiation offer.
       *
       * @return {Object} Extension parameters
       * @public
       */
      offer() {
        const params = {};
        if (this._options.serverNoContextTakeover) {
          params.server_no_context_takeover = true;
        }
        if (this._options.clientNoContextTakeover) {
          params.client_no_context_takeover = true;
        }
        if (this._options.serverMaxWindowBits) {
          params.server_max_window_bits = this._options.serverMaxWindowBits;
        }
        if (this._options.clientMaxWindowBits) {
          params.client_max_window_bits = this._options.clientMaxWindowBits;
        } else if (this._options.clientMaxWindowBits == null) {
          params.client_max_window_bits = true;
        }
        return params;
      }
      /**
       * Accept an extension negotiation offer/response.
       *
       * @param {Array} configurations The extension negotiation offers/reponse
       * @return {Object} Accepted configuration
       * @public
       */
      accept(configurations) {
        configurations = this.normalizeParams(configurations);
        this.params = this._isServer ? this.acceptAsServer(configurations) : this.acceptAsClient(configurations);
        return this.params;
      }
      /**
       * Releases all resources used by the extension.
       *
       * @public
       */
      cleanup() {
        if (this._inflate) {
          this._inflate.close();
          this._inflate = null;
        }
        if (this._deflate) {
          const callback = this._deflate[kCallback];
          this._deflate.close();
          this._deflate = null;
          if (callback) {
            callback(
              new Error(
                "The deflate stream was closed while data was being processed"
              )
            );
          }
        }
      }
      /**
       *  Accept an extension negotiation offer.
       *
       * @param {Array} offers The extension negotiation offers
       * @return {Object} Accepted configuration
       * @private
       */
      acceptAsServer(offers) {
        const opts = this._options;
        const accepted = offers.find((params) => {
          if (opts.serverNoContextTakeover === false && params.server_no_context_takeover || params.server_max_window_bits && (opts.serverMaxWindowBits === false || typeof opts.serverMaxWindowBits === "number" && opts.serverMaxWindowBits > params.server_max_window_bits) || typeof opts.clientMaxWindowBits === "number" && !params.client_max_window_bits) {
            return false;
          }
          return true;
        });
        if (!accepted) {
          throw new Error("None of the extension offers can be accepted");
        }
        if (opts.serverNoContextTakeover) {
          accepted.server_no_context_takeover = true;
        }
        if (opts.clientNoContextTakeover) {
          accepted.client_no_context_takeover = true;
        }
        if (typeof opts.serverMaxWindowBits === "number") {
          accepted.server_max_window_bits = opts.serverMaxWindowBits;
        }
        if (typeof opts.clientMaxWindowBits === "number") {
          accepted.client_max_window_bits = opts.clientMaxWindowBits;
        } else if (accepted.client_max_window_bits === true || opts.clientMaxWindowBits === false) {
          delete accepted.client_max_window_bits;
        }
        return accepted;
      }
      /**
       * Accept the extension negotiation response.
       *
       * @param {Array} response The extension negotiation response
       * @return {Object} Accepted configuration
       * @private
       */
      acceptAsClient(response) {
        const params = response[0];
        if (this._options.clientNoContextTakeover === false && params.client_no_context_takeover) {
          throw new Error('Unexpected parameter "client_no_context_takeover"');
        }
        if (!params.client_max_window_bits) {
          if (typeof this._options.clientMaxWindowBits === "number") {
            params.client_max_window_bits = this._options.clientMaxWindowBits;
          }
        } else if (this._options.clientMaxWindowBits === false || typeof this._options.clientMaxWindowBits === "number" && params.client_max_window_bits > this._options.clientMaxWindowBits) {
          throw new Error(
            'Unexpected or invalid parameter "client_max_window_bits"'
          );
        }
        return params;
      }
      /**
       * Normalize parameters.
       *
       * @param {Array} configurations The extension negotiation offers/reponse
       * @return {Array} The offers/response with normalized parameters
       * @private
       */
      normalizeParams(configurations) {
        configurations.forEach((params) => {
          Object.keys(params).forEach((key) => {
            let value = params[key];
            if (value.length > 1) {
              throw new Error(`Parameter "${key}" must have only a single value`);
            }
            value = value[0];
            if (key === "client_max_window_bits") {
              if (value !== true) {
                const num = +value;
                if (!Number.isInteger(num) || num < 8 || num > 15) {
                  throw new TypeError(
                    `Invalid value for parameter "${key}": ${value}`
                  );
                }
                value = num;
              } else if (!this._isServer) {
                throw new TypeError(
                  `Invalid value for parameter "${key}": ${value}`
                );
              }
            } else if (key === "server_max_window_bits") {
              const num = +value;
              if (!Number.isInteger(num) || num < 8 || num > 15) {
                throw new TypeError(
                  `Invalid value for parameter "${key}": ${value}`
                );
              }
              value = num;
            } else if (key === "client_no_context_takeover" || key === "server_no_context_takeover") {
              if (value !== true) {
                throw new TypeError(
                  `Invalid value for parameter "${key}": ${value}`
                );
              }
            } else {
              throw new Error(`Unknown parameter "${key}"`);
            }
            params[key] = value;
          });
        });
        return configurations;
      }
      /**
       * Decompress data. Concurrency limited.
       *
       * @param {Buffer} data Compressed data
       * @param {Boolean} fin Specifies whether or not this is the last fragment
       * @param {Function} callback Callback
       * @public
       */
      decompress(data, fin, callback) {
        zlibLimiter.add((done) => {
          this._decompress(data, fin, (err, result) => {
            done();
            callback(err, result);
          });
        });
      }
      /**
       * Compress data. Concurrency limited.
       *
       * @param {(Buffer|String)} data Data to compress
       * @param {Boolean} fin Specifies whether or not this is the last fragment
       * @param {Function} callback Callback
       * @public
       */
      compress(data, fin, callback) {
        zlibLimiter.add((done) => {
          this._compress(data, fin, (err, result) => {
            done();
            callback(err, result);
          });
        });
      }
      /**
       * Decompress data.
       *
       * @param {Buffer} data Compressed data
       * @param {Boolean} fin Specifies whether or not this is the last fragment
       * @param {Function} callback Callback
       * @private
       */
      _decompress(data, fin, callback) {
        const endpoint = this._isServer ? "client" : "server";
        if (!this._inflate) {
          const key = `${endpoint}_max_window_bits`;
          const windowBits = typeof this.params[key] !== "number" ? zlib.Z_DEFAULT_WINDOWBITS : this.params[key];
          this._inflate = zlib.createInflateRaw({
            ...this._options.zlibInflateOptions,
            windowBits
          });
          this._inflate[kPerMessageDeflate] = this;
          this._inflate[kTotalLength] = 0;
          this._inflate[kBuffers] = [];
          this._inflate.on("error", inflateOnError);
          this._inflate.on("data", inflateOnData);
        }
        this._inflate[kCallback] = callback;
        this._inflate.write(data);
        if (fin) {this._inflate.write(TRAILER);}
        this._inflate.flush(() => {
          const err = this._inflate[kError];
          if (err) {
            this._inflate.close();
            this._inflate = null;
            callback(err);
            return;
          }
          const data2 = bufferUtil.concat(
            this._inflate[kBuffers],
            this._inflate[kTotalLength]
          );
          if (this._inflate._readableState.endEmitted) {
            this._inflate.close();
            this._inflate = null;
          } else {
            this._inflate[kTotalLength] = 0;
            this._inflate[kBuffers] = [];
            if (fin && this.params[`${endpoint}_no_context_takeover`]) {
              this._inflate.reset();
            }
          }
          callback(null, data2);
        });
      }
      /**
       * Compress data.
       *
       * @param {(Buffer|String)} data Data to compress
       * @param {Boolean} fin Specifies whether or not this is the last fragment
       * @param {Function} callback Callback
       * @private
       */
      _compress(data, fin, callback) {
        const endpoint = this._isServer ? "server" : "client";
        if (!this._deflate) {
          const key = `${endpoint}_max_window_bits`;
          const windowBits = typeof this.params[key] !== "number" ? zlib.Z_DEFAULT_WINDOWBITS : this.params[key];
          this._deflate = zlib.createDeflateRaw({
            ...this._options.zlibDeflateOptions,
            windowBits
          });
          this._deflate[kTotalLength] = 0;
          this._deflate[kBuffers] = [];
          this._deflate.on("data", deflateOnData);
        }
        this._deflate[kCallback] = callback;
        this._deflate.write(data);
        this._deflate.flush(zlib.Z_SYNC_FLUSH, () => {
          if (!this._deflate) {
            return;
          }
          let data2 = bufferUtil.concat(
            this._deflate[kBuffers],
            this._deflate[kTotalLength]
          );
          if (fin) {
            data2 = new FastBuffer(data2.buffer, data2.byteOffset, data2.length - 4);
          }
          this._deflate[kCallback] = null;
          this._deflate[kTotalLength] = 0;
          this._deflate[kBuffers] = [];
          if (fin && this.params[`${endpoint}_no_context_takeover`]) {
            this._deflate.reset();
          }
          callback(null, data2);
        });
      }
    };
    module2.exports = PerMessageDeflate2;
    function deflateOnData(chunk) {
      this[kBuffers].push(chunk);
      this[kTotalLength] += chunk.length;
    }
    function inflateOnData(chunk) {
      this[kTotalLength] += chunk.length;
      if (this[kPerMessageDeflate]._maxPayload < 1 || this[kTotalLength] <= this[kPerMessageDeflate]._maxPayload) {
        this[kBuffers].push(chunk);
        return;
      }
      this[kError] = new RangeError("Max payload size exceeded");
      this[kError].code = "WS_ERR_UNSUPPORTED_MESSAGE_LENGTH";
      this[kError][kStatusCode] = 1009;
      this.removeListener("data", inflateOnData);
      this.reset();
    }
    function inflateOnError(err) {
      this[kPerMessageDeflate]._inflate = null;
      if (this[kError]) {
        this[kCallback](this[kError]);
        return;
      }
      err[kStatusCode] = 1007;
      this[kCallback](err);
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/validation.js
var require_validation = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/validation.js"(exports2, module2) {
    "use strict";
    var { isUtf8 } = require("node:buffer");
    var { hasBlob } = require_constants();
    var tokenChars = [
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      // 0 - 15
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      // 16 - 31
      0,
      1,
      0,
      1,
      1,
      1,
      1,
      1,
      0,
      0,
      1,
      1,
      0,
      1,
      1,
      0,
      // 32 - 47
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      0,
      0,
      0,
      0,
      0,
      0,
      // 48 - 63
      0,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      // 64 - 79
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      0,
      0,
      0,
      1,
      1,
      // 80 - 95
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      // 96 - 111
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      1,
      0,
      1,
      0,
      1,
      0
      // 112 - 127
    ];
    function isValidStatusCode(code) {
      return code >= 1e3 && code <= 1014 && code !== 1004 && code !== 1005 && code !== 1006 || code >= 3e3 && code <= 4999;
    }
    function _isValidUTF8(buf) {
      const len = buf.length;
      let i = 0;
      while (i < len) {
        if ((buf[i] & 128) === 0) {
          i++;
        } else if ((buf[i] & 224) === 192) {
          if (i + 1 === len || (buf[i + 1] & 192) !== 128 || (buf[i] & 254) === 192) {
            return false;
          }
          i += 2;
        } else if ((buf[i] & 240) === 224) {
          if (i + 2 >= len || (buf[i + 1] & 192) !== 128 || (buf[i + 2] & 192) !== 128 || buf[i] === 224 && (buf[i + 1] & 224) === 128 || // Overlong
          buf[i] === 237 && (buf[i + 1] & 224) === 160) {
            return false;
          }
          i += 3;
        } else if ((buf[i] & 248) === 240) {
          if (i + 3 >= len || (buf[i + 1] & 192) !== 128 || (buf[i + 2] & 192) !== 128 || (buf[i + 3] & 192) !== 128 || buf[i] === 240 && (buf[i + 1] & 240) === 128 || // Overlong
          buf[i] === 244 && buf[i + 1] > 143 || buf[i] > 244) {
            return false;
          }
          i += 4;
        } else {
          return false;
        }
      }
      return true;
    }
    function isBlob(value) {
      return hasBlob && typeof value === "object" && typeof value.arrayBuffer === "function" && typeof value.type === "string" && typeof value.stream === "function" && (value[Symbol.toStringTag] === "Blob" || value[Symbol.toStringTag] === "File");
    }
    module2.exports = {
      isBlob,
      isValidStatusCode,
      isValidUTF8: _isValidUTF8,
      tokenChars
    };
    if (isUtf8) {
      module2.exports.isValidUTF8 = function(buf) {
        return buf.length < 24 ? _isValidUTF8(buf) : isUtf8(buf);
      };
    } else if (!process.env.WS_NO_UTF_8_VALIDATE) {
      try {
        const isValidUTF8 = require("utf-8-validate");
        module2.exports.isValidUTF8 = function(buf) {
          return buf.length < 32 ? _isValidUTF8(buf) : isValidUTF8(buf);
        };
      } catch (e) {
      }
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/receiver.js
var require_receiver = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/receiver.js"(exports2, module2) {
    "use strict";
    var { Writable } = require("node:stream");
    var PerMessageDeflate2 = require_permessage_deflate();
    var {
      BINARY_TYPES,
      EMPTY_BUFFER,
      kStatusCode,
      kWebSocket
    } = require_constants();
    var { concat, toArrayBuffer, unmask } = require_buffer_util();
    var { isValidStatusCode, isValidUTF8 } = require_validation();
    var FastBuffer = Buffer[Symbol.species];
    var GET_INFO = 0;
    var GET_PAYLOAD_LENGTH_16 = 1;
    var GET_PAYLOAD_LENGTH_64 = 2;
    var GET_MASK = 3;
    var GET_DATA = 4;
    var INFLATING = 5;
    var DEFER_EVENT = 6;
    var Receiver2 = class extends Writable {
      /**
       * Creates a Receiver instance.
       *
       * @param {Object} [options] Options object
       * @param {Boolean} [options.allowSynchronousEvents=true] Specifies whether
       *     any of the `'message'`, `'ping'`, and `'pong'` events can be emitted
       *     multiple times in the same tick
       * @param {String} [options.binaryType=nodebuffer] The type for binary data
       * @param {Object} [options.extensions] An object containing the negotiated
       *     extensions
       * @param {Boolean} [options.isServer=false] Specifies whether to operate in
       *     client or server mode
       * @param {Number} [options.maxBufferedChunks=0] The maximum number of
       *     buffered data chunks
       * @param {Number} [options.maxFragments=0] The maximum number of message
       *     fragments
       * @param {Number} [options.maxPayload=0] The maximum allowed message length
       * @param {Boolean} [options.skipUTF8Validation=false] Specifies whether or
       *     not to skip UTF-8 validation for text and close messages
       */
      constructor(options = {}) {
        super();
        this._allowSynchronousEvents = options.allowSynchronousEvents !== void 0 ? options.allowSynchronousEvents : true;
        this._binaryType = options.binaryType || BINARY_TYPES[0];
        this._extensions = options.extensions || {};
        this._isServer = !!options.isServer;
        this._maxBufferedChunks = options.maxBufferedChunks | 0;
        this._maxFragments = options.maxFragments | 0;
        this._maxPayload = options.maxPayload | 0;
        this._skipUTF8Validation = !!options.skipUTF8Validation;
        this[kWebSocket] = void 0;
        this._bufferedBytes = 0;
        this._buffers = [];
        this._compressed = false;
        this._payloadLength = 0;
        this._mask = void 0;
        this._fragmented = 0;
        this._masked = false;
        this._fin = false;
        this._opcode = 0;
        this._totalPayloadLength = 0;
        this._messageLength = 0;
        this._fragments = [];
        this._errored = false;
        this._loop = false;
        this._state = GET_INFO;
      }
      /**
       * Implements `Writable.prototype._write()`.
       *
       * @param {Buffer} chunk The chunk of data to write
       * @param {String} encoding The character encoding of `chunk`
       * @param {Function} cb Callback
       * @private
       */
      _write(chunk, encoding, cb) {
        if (this._opcode === 8 && this._state == GET_INFO) {return cb();}
        if (this._maxBufferedChunks > 0 && this._buffers.length >= this._maxBufferedChunks) {
          cb(
            this.createError(
              RangeError,
              "Too many buffered chunks",
              false,
              1008,
              "WS_ERR_TOO_MANY_BUFFERED_PARTS"
            )
          );
          return;
        }
        this._bufferedBytes += chunk.length;
        this._buffers.push(chunk);
        this.startLoop(cb);
      }
      /**
       * Consumes `n` bytes from the buffered data.
       *
       * @param {Number} n The number of bytes to consume
       * @return {Buffer} The consumed bytes
       * @private
       */
      consume(n) {
        this._bufferedBytes -= n;
        if (n === this._buffers[0].length) {return this._buffers.shift();}
        if (n < this._buffers[0].length) {
          const buf = this._buffers[0];
          this._buffers[0] = new FastBuffer(
            buf.buffer,
            buf.byteOffset + n,
            buf.length - n
          );
          return new FastBuffer(buf.buffer, buf.byteOffset, n);
        }
        const dst = Buffer.allocUnsafe(n);
        do {
          const buf = this._buffers[0];
          const offset = dst.length - n;
          if (n >= buf.length) {
            dst.set(this._buffers.shift(), offset);
          } else {
            dst.set(new Uint8Array(buf.buffer, buf.byteOffset, n), offset);
            this._buffers[0] = new FastBuffer(
              buf.buffer,
              buf.byteOffset + n,
              buf.length - n
            );
          }
          n -= buf.length;
        } while (n > 0);
        return dst;
      }
      /**
       * Starts the parsing loop.
       *
       * @param {Function} cb Callback
       * @private
       */
      startLoop(cb) {
        this._loop = true;
        do {
          switch (this._state) {
            case GET_INFO:
              this.getInfo(cb);
              break;
            case GET_PAYLOAD_LENGTH_16:
              this.getPayloadLength16(cb);
              break;
            case GET_PAYLOAD_LENGTH_64:
              this.getPayloadLength64(cb);
              break;
            case GET_MASK:
              this.getMask();
              break;
            case GET_DATA:
              this.getData(cb);
              break;
            case INFLATING:
            case DEFER_EVENT:
              this._loop = false;
              return;
          }
        } while (this._loop);
        if (!this._errored) {cb();}
      }
      /**
       * Reads the first two bytes of a frame.
       *
       * @param {Function} cb Callback
       * @private
       */
      getInfo(cb) {
        if (this._bufferedBytes < 2) {
          this._loop = false;
          return;
        }
        const buf = this.consume(2);
        if ((buf[0] & 48) !== 0) {
          const error = this.createError(
            RangeError,
            "RSV2 and RSV3 must be clear",
            true,
            1002,
            "WS_ERR_UNEXPECTED_RSV_2_3"
          );
          cb(error);
          return;
        }
        const compressed = (buf[0] & 64) === 64;
        if (compressed && !this._extensions[PerMessageDeflate2.extensionName]) {
          const error = this.createError(
            RangeError,
            "RSV1 must be clear",
            true,
            1002,
            "WS_ERR_UNEXPECTED_RSV_1"
          );
          cb(error);
          return;
        }
        this._fin = (buf[0] & 128) === 128;
        this._opcode = buf[0] & 15;
        this._payloadLength = buf[1] & 127;
        if (this._opcode === 0) {
          if (compressed) {
            const error = this.createError(
              RangeError,
              "RSV1 must be clear",
              true,
              1002,
              "WS_ERR_UNEXPECTED_RSV_1"
            );
            cb(error);
            return;
          }
          if (!this._fragmented) {
            const error = this.createError(
              RangeError,
              "invalid opcode 0",
              true,
              1002,
              "WS_ERR_INVALID_OPCODE"
            );
            cb(error);
            return;
          }
          this._opcode = this._fragmented;
        } else if (this._opcode === 1 || this._opcode === 2) {
          if (this._fragmented) {
            const error = this.createError(
              RangeError,
              `invalid opcode ${this._opcode}`,
              true,
              1002,
              "WS_ERR_INVALID_OPCODE"
            );
            cb(error);
            return;
          }
          this._compressed = compressed;
        } else if (this._opcode > 7 && this._opcode < 11) {
          if (!this._fin) {
            const error = this.createError(
              RangeError,
              "FIN must be set",
              true,
              1002,
              "WS_ERR_EXPECTED_FIN"
            );
            cb(error);
            return;
          }
          if (compressed) {
            const error = this.createError(
              RangeError,
              "RSV1 must be clear",
              true,
              1002,
              "WS_ERR_UNEXPECTED_RSV_1"
            );
            cb(error);
            return;
          }
          if (this._payloadLength > 125 || this._opcode === 8 && this._payloadLength === 1) {
            const error = this.createError(
              RangeError,
              `invalid payload length ${this._payloadLength}`,
              true,
              1002,
              "WS_ERR_INVALID_CONTROL_PAYLOAD_LENGTH"
            );
            cb(error);
            return;
          }
        } else {
          const error = this.createError(
            RangeError,
            `invalid opcode ${this._opcode}`,
            true,
            1002,
            "WS_ERR_INVALID_OPCODE"
          );
          cb(error);
          return;
        }
        if (!this._fin && !this._fragmented) {this._fragmented = this._opcode;}
        this._masked = (buf[1] & 128) === 128;
        if (this._isServer) {
          if (!this._masked) {
            const error = this.createError(
              RangeError,
              "MASK must be set",
              true,
              1002,
              "WS_ERR_EXPECTED_MASK"
            );
            cb(error);
            return;
          }
        } else if (this._masked) {
          const error = this.createError(
            RangeError,
            "MASK must be clear",
            true,
            1002,
            "WS_ERR_UNEXPECTED_MASK"
          );
          cb(error);
          return;
        }
        if (this._payloadLength === 126) {this._state = GET_PAYLOAD_LENGTH_16;}
        else if (this._payloadLength === 127) {this._state = GET_PAYLOAD_LENGTH_64;}
        else {this.haveLength(cb);}
      }
      /**
       * Gets extended payload length (7+16).
       *
       * @param {Function} cb Callback
       * @private
       */
      getPayloadLength16(cb) {
        if (this._bufferedBytes < 2) {
          this._loop = false;
          return;
        }
        this._payloadLength = this.consume(2).readUInt16BE(0);
        this.haveLength(cb);
      }
      /**
       * Gets extended payload length (7+64).
       *
       * @param {Function} cb Callback
       * @private
       */
      getPayloadLength64(cb) {
        if (this._bufferedBytes < 8) {
          this._loop = false;
          return;
        }
        const buf = this.consume(8);
        const num = buf.readUInt32BE(0);
        if (num > Math.pow(2, 53 - 32) - 1) {
          const error = this.createError(
            RangeError,
            "Unsupported WebSocket frame: payload length > 2^53 - 1",
            false,
            1009,
            "WS_ERR_UNSUPPORTED_DATA_PAYLOAD_LENGTH"
          );
          cb(error);
          return;
        }
        this._payloadLength = num * Math.pow(2, 32) + buf.readUInt32BE(4);
        this.haveLength(cb);
      }
      /**
       * Payload length has been read.
       *
       * @param {Function} cb Callback
       * @private
       */
      haveLength(cb) {
        if (this._payloadLength && this._opcode < 8) {
          this._totalPayloadLength += this._payloadLength;
          if (this._totalPayloadLength > this._maxPayload && this._maxPayload > 0) {
            const error = this.createError(
              RangeError,
              "Max payload size exceeded",
              false,
              1009,
              "WS_ERR_UNSUPPORTED_MESSAGE_LENGTH"
            );
            cb(error);
            return;
          }
        }
        if (this._masked) {this._state = GET_MASK;}
        else {this._state = GET_DATA;}
      }
      /**
       * Reads mask bytes.
       *
       * @private
       */
      getMask() {
        if (this._bufferedBytes < 4) {
          this._loop = false;
          return;
        }
        this._mask = this.consume(4);
        this._state = GET_DATA;
      }
      /**
       * Reads data bytes.
       *
       * @param {Function} cb Callback
       * @private
       */
      getData(cb) {
        let data = EMPTY_BUFFER;
        if (this._payloadLength) {
          if (this._bufferedBytes < this._payloadLength) {
            this._loop = false;
            return;
          }
          data = this.consume(this._payloadLength);
          if (this._masked && (this._mask[0] | this._mask[1] | this._mask[2] | this._mask[3]) !== 0) {
            unmask(data, this._mask);
          }
        }
        if (this._opcode > 7) {
          this.controlMessage(data, cb);
          return;
        }
        if (this._compressed) {
          this._state = INFLATING;
          this.decompress(data, cb);
          return;
        }
        if (data.length) {
          if (this._maxFragments > 0 && this._fragments.length >= this._maxFragments) {
            const error = this.createError(
              RangeError,
              "Too many message fragments",
              false,
              1008,
              "WS_ERR_TOO_MANY_BUFFERED_PARTS"
            );
            cb(error);
            return;
          }
          this._messageLength = this._totalPayloadLength;
          this._fragments.push(data);
        }
        this.dataMessage(cb);
      }
      /**
       * Decompresses data.
       *
       * @param {Buffer} data Compressed data
       * @param {Function} cb Callback
       * @private
       */
      decompress(data, cb) {
        const perMessageDeflate = this._extensions[PerMessageDeflate2.extensionName];
        perMessageDeflate.decompress(data, this._fin, (err, buf) => {
          if (err) {return cb(err);}
          if (buf.length) {
            this._messageLength += buf.length;
            if (this._messageLength > this._maxPayload && this._maxPayload > 0) {
              const error = this.createError(
                RangeError,
                "Max payload size exceeded",
                false,
                1009,
                "WS_ERR_UNSUPPORTED_MESSAGE_LENGTH"
              );
              cb(error);
              return;
            }
            if (this._maxFragments > 0 && this._fragments.length >= this._maxFragments) {
              const error = this.createError(
                RangeError,
                "Too many message fragments",
                false,
                1008,
                "WS_ERR_TOO_MANY_BUFFERED_PARTS"
              );
              cb(error);
              return;
            }
            this._fragments.push(buf);
          }
          this.dataMessage(cb);
          if (this._state === GET_INFO) {this.startLoop(cb);}
        });
      }
      /**
       * Handles a data message.
       *
       * @param {Function} cb Callback
       * @private
       */
      dataMessage(cb) {
        if (!this._fin) {
          this._state = GET_INFO;
          return;
        }
        const messageLength = this._messageLength;
        const fragments = this._fragments;
        this._totalPayloadLength = 0;
        this._messageLength = 0;
        this._fragmented = 0;
        this._fragments = [];
        if (this._opcode === 2) {
          let data;
          if (this._binaryType === "nodebuffer") {
            data = concat(fragments, messageLength);
          } else if (this._binaryType === "arraybuffer") {
            data = toArrayBuffer(concat(fragments, messageLength));
          } else if (this._binaryType === "blob") {
            data = new Blob(fragments);
          } else {
            data = fragments;
          }
          if (this._allowSynchronousEvents) {
            this.emit("message", data, true);
            this._state = GET_INFO;
          } else {
            this._state = DEFER_EVENT;
            setImmediate(() => {
              this.emit("message", data, true);
              this._state = GET_INFO;
              this.startLoop(cb);
            });
          }
        } else {
          const buf = concat(fragments, messageLength);
          if (!this._skipUTF8Validation && !isValidUTF8(buf)) {
            const error = this.createError(
              Error,
              "invalid UTF-8 sequence",
              true,
              1007,
              "WS_ERR_INVALID_UTF8"
            );
            cb(error);
            return;
          }
          if (this._state === INFLATING || this._allowSynchronousEvents) {
            this.emit("message", buf, false);
            this._state = GET_INFO;
          } else {
            this._state = DEFER_EVENT;
            setImmediate(() => {
              this.emit("message", buf, false);
              this._state = GET_INFO;
              this.startLoop(cb);
            });
          }
        }
      }
      /**
       * Handles a control message.
       *
       * @param {Buffer} data Data to handle
       * @return {(Error|RangeError|undefined)} A possible error
       * @private
       */
      controlMessage(data, cb) {
        if (this._opcode === 8) {
          if (data.length === 0) {
            this._loop = false;
            this.emit("conclude", 1005, EMPTY_BUFFER);
            this.end();
          } else {
            const code = data.readUInt16BE(0);
            if (!isValidStatusCode(code)) {
              const error = this.createError(
                RangeError,
                `invalid status code ${code}`,
                true,
                1002,
                "WS_ERR_INVALID_CLOSE_CODE"
              );
              cb(error);
              return;
            }
            const buf = new FastBuffer(
              data.buffer,
              data.byteOffset + 2,
              data.length - 2
            );
            if (!this._skipUTF8Validation && !isValidUTF8(buf)) {
              const error = this.createError(
                Error,
                "invalid UTF-8 sequence",
                true,
                1007,
                "WS_ERR_INVALID_UTF8"
              );
              cb(error);
              return;
            }
            this._loop = false;
            this.emit("conclude", code, buf);
            this.end();
          }
          this._state = GET_INFO;
          return;
        }
        if (this._allowSynchronousEvents) {
          this.emit(this._opcode === 9 ? "ping" : "pong", data);
          this._state = GET_INFO;
        } else {
          this._state = DEFER_EVENT;
          setImmediate(() => {
            this.emit(this._opcode === 9 ? "ping" : "pong", data);
            this._state = GET_INFO;
            this.startLoop(cb);
          });
        }
      }
      /**
       * Builds an error object.
       *
       * @param {function(new:Error|RangeError)} ErrorCtor The error constructor
       * @param {String} message The error message
       * @param {Boolean} prefix Specifies whether or not to add a default prefix to
       *     `message`
       * @param {Number} statusCode The status code
       * @param {String} errorCode The exposed error code
       * @return {(Error|RangeError)} The error
       * @private
       */
      createError(ErrorCtor, message, prefix, statusCode, errorCode) {
        this._loop = false;
        this._errored = true;
        const err = new ErrorCtor(
          prefix ? `Invalid WebSocket frame: ${message}` : message
        );
        Error.captureStackTrace(err, this.createError);
        err.code = errorCode;
        err[kStatusCode] = statusCode;
        return err;
      }
    };
    module2.exports = Receiver2;
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/sender.js
var require_sender = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/sender.js"(exports2, module2) {
    "use strict";
    var { Duplex } = require("node:stream");
    var { randomFillSync } = require("node:crypto");
    var {
      types: { isUint8Array }
    } = require("node:util");
    var PerMessageDeflate2 = require_permessage_deflate();
    var { EMPTY_BUFFER, kWebSocket, NOOP } = require_constants();
    var { isBlob, isValidStatusCode } = require_validation();
    var { mask: applyMask, toBuffer } = require_buffer_util();
    var kByteLength = /* @__PURE__ */ Symbol("kByteLength");
    var maskBuffer = Buffer.alloc(4);
    var RANDOM_POOL_SIZE = 8 * 1024;
    var randomPool;
    var randomPoolPointer = RANDOM_POOL_SIZE;
    var DEFAULT = 0;
    var DEFLATING = 1;
    var GET_BLOB_DATA = 2;
    var Sender2 = class _Sender {
      /**
       * Creates a Sender instance.
       *
       * @param {Duplex} socket The connection socket
       * @param {Object} [extensions] An object containing the negotiated extensions
       * @param {Function} [generateMask] The function used to generate the masking
       *     key
       */
      constructor(socket2, extensions, generateMask) {
        this._extensions = extensions || {};
        if (generateMask) {
          this._generateMask = generateMask;
          this._maskBuffer = Buffer.alloc(4);
        }
        this._socket = socket2;
        this._firstFragment = true;
        this._compress = false;
        this._bufferedBytes = 0;
        this._queue = [];
        this._state = DEFAULT;
        this.onerror = NOOP;
        this[kWebSocket] = void 0;
      }
      /**
       * Frames a piece of data according to the HyBi WebSocket protocol.
       *
       * @param {(Buffer|String)} data The data to frame
       * @param {Object} options Options object
       * @param {Boolean} [options.fin=false] Specifies whether or not to set the
       *     FIN bit
       * @param {Function} [options.generateMask] The function used to generate the
       *     masking key
       * @param {Boolean} [options.mask=false] Specifies whether or not to mask
       *     `data`
       * @param {Buffer} [options.maskBuffer] The buffer used to store the masking
       *     key
       * @param {Number} options.opcode The opcode
       * @param {Boolean} [options.readOnly=false] Specifies whether `data` can be
       *     modified
       * @param {Boolean} [options.rsv1=false] Specifies whether or not to set the
       *     RSV1 bit
       * @return {(Buffer|String)[]} The framed data
       * @public
       */
      static frame(data, options) {
        let mask;
        let merge = false;
        let offset = 2;
        let skipMasking = false;
        if (options.mask) {
          mask = options.maskBuffer || maskBuffer;
          if (options.generateMask) {
            options.generateMask(mask);
          } else {
            if (randomPoolPointer === RANDOM_POOL_SIZE) {
              if (randomPool === void 0) {
                randomPool = Buffer.alloc(RANDOM_POOL_SIZE);
              }
              randomFillSync(randomPool, 0, RANDOM_POOL_SIZE);
              randomPoolPointer = 0;
            }
            mask[0] = randomPool[randomPoolPointer++];
            mask[1] = randomPool[randomPoolPointer++];
            mask[2] = randomPool[randomPoolPointer++];
            mask[3] = randomPool[randomPoolPointer++];
          }
          skipMasking = (mask[0] | mask[1] | mask[2] | mask[3]) === 0;
          offset = 6;
        }
        let dataLength;
        if (typeof data === "string") {
          if ((!options.mask || skipMasking) && options[kByteLength] !== void 0) {
            dataLength = options[kByteLength];
          } else {
            data = Buffer.from(data);
            dataLength = data.length;
          }
        } else {
          dataLength = data.length;
          merge = options.mask && options.readOnly && !skipMasking;
        }
        let payloadLength = dataLength;
        if (dataLength >= 65536) {
          offset += 8;
          payloadLength = 127;
        } else if (dataLength > 125) {
          offset += 2;
          payloadLength = 126;
        }
        const target = Buffer.allocUnsafe(merge ? dataLength + offset : offset);
        target[0] = options.fin ? options.opcode | 128 : options.opcode;
        if (options.rsv1) {target[0] |= 64;}
        target[1] = payloadLength;
        if (payloadLength === 126) {
          target.writeUInt16BE(dataLength, 2);
        } else if (payloadLength === 127) {
          target[2] = target[3] = 0;
          target.writeUIntBE(dataLength, 4, 6);
        }
        if (!options.mask) {return [target, data];}
        target[1] |= 128;
        target[offset - 4] = mask[0];
        target[offset - 3] = mask[1];
        target[offset - 2] = mask[2];
        target[offset - 1] = mask[3];
        if (skipMasking) {return [target, data];}
        if (merge) {
          applyMask(data, mask, target, offset, dataLength);
          return [target];
        }
        applyMask(data, mask, data, 0, dataLength);
        return [target, data];
      }
      /**
       * Sends a close message to the other peer.
       *
       * @param {Number} [code] The status code component of the body
       * @param {(String|Buffer)} [data] The message component of the body
       * @param {Boolean} [mask=false] Specifies whether or not to mask the message
       * @param {Function} [cb] Callback
       * @public
       */
      close(code, data, mask, cb) {
        let buf;
        if (code === void 0) {
          buf = EMPTY_BUFFER;
        } else if (typeof code !== "number" || !isValidStatusCode(code)) {
          throw new TypeError("First argument must be a valid error code number");
        } else if (data === void 0 || !data.length) {
          buf = Buffer.allocUnsafe(2);
          buf.writeUInt16BE(code, 0);
        } else {
          const length = Buffer.byteLength(data);
          if (length > 123) {
            throw new RangeError("The message must not be greater than 123 bytes");
          }
          buf = Buffer.allocUnsafe(2 + length);
          buf.writeUInt16BE(code, 0);
          if (typeof data === "string") {
            buf.write(data, 2);
          } else if (isUint8Array(data)) {
            buf.set(data, 2);
          } else {
            throw new TypeError("Second argument must be a string or a Uint8Array");
          }
        }
        const options = {
          [kByteLength]: buf.length,
          fin: true,
          generateMask: this._generateMask,
          mask,
          maskBuffer: this._maskBuffer,
          opcode: 8,
          readOnly: false,
          rsv1: false
        };
        if (this._state !== DEFAULT) {
          this.enqueue([this.dispatch, buf, false, options, cb]);
        } else {
          this.sendFrame(_Sender.frame(buf, options), cb);
        }
      }
      /**
       * Sends a ping message to the other peer.
       *
       * @param {*} data The message to send
       * @param {Boolean} [mask=false] Specifies whether or not to mask `data`
       * @param {Function} [cb] Callback
       * @public
       */
      ping(data, mask, cb) {
        let byteLength;
        let readOnly;
        if (typeof data === "string") {
          byteLength = Buffer.byteLength(data);
          readOnly = false;
        } else if (isBlob(data)) {
          byteLength = data.size;
          readOnly = false;
        } else {
          data = toBuffer(data);
          byteLength = data.length;
          readOnly = toBuffer.readOnly;
        }
        if (byteLength > 125) {
          throw new RangeError("The data size must not be greater than 125 bytes");
        }
        const options = {
          [kByteLength]: byteLength,
          fin: true,
          generateMask: this._generateMask,
          mask,
          maskBuffer: this._maskBuffer,
          opcode: 9,
          readOnly,
          rsv1: false
        };
        if (isBlob(data)) {
          if (this._state !== DEFAULT) {
            this.enqueue([this.getBlobData, data, false, options, cb]);
          } else {
            this.getBlobData(data, false, options, cb);
          }
        } else if (this._state !== DEFAULT) {
          this.enqueue([this.dispatch, data, false, options, cb]);
        } else {
          this.sendFrame(_Sender.frame(data, options), cb);
        }
      }
      /**
       * Sends a pong message to the other peer.
       *
       * @param {*} data The message to send
       * @param {Boolean} [mask=false] Specifies whether or not to mask `data`
       * @param {Function} [cb] Callback
       * @public
       */
      pong(data, mask, cb) {
        let byteLength;
        let readOnly;
        if (typeof data === "string") {
          byteLength = Buffer.byteLength(data);
          readOnly = false;
        } else if (isBlob(data)) {
          byteLength = data.size;
          readOnly = false;
        } else {
          data = toBuffer(data);
          byteLength = data.length;
          readOnly = toBuffer.readOnly;
        }
        if (byteLength > 125) {
          throw new RangeError("The data size must not be greater than 125 bytes");
        }
        const options = {
          [kByteLength]: byteLength,
          fin: true,
          generateMask: this._generateMask,
          mask,
          maskBuffer: this._maskBuffer,
          opcode: 10,
          readOnly,
          rsv1: false
        };
        if (isBlob(data)) {
          if (this._state !== DEFAULT) {
            this.enqueue([this.getBlobData, data, false, options, cb]);
          } else {
            this.getBlobData(data, false, options, cb);
          }
        } else if (this._state !== DEFAULT) {
          this.enqueue([this.dispatch, data, false, options, cb]);
        } else {
          this.sendFrame(_Sender.frame(data, options), cb);
        }
      }
      /**
       * Sends a data message to the other peer.
       *
       * @param {*} data The message to send
       * @param {Object} options Options object
       * @param {Boolean} [options.binary=false] Specifies whether `data` is binary
       *     or text
       * @param {Boolean} [options.compress=false] Specifies whether or not to
       *     compress `data`
       * @param {Boolean} [options.fin=false] Specifies whether the fragment is the
       *     last one
       * @param {Boolean} [options.mask=false] Specifies whether or not to mask
       *     `data`
       * @param {Function} [cb] Callback
       * @public
       */
      send(data, options, cb) {
        const perMessageDeflate = this._extensions[PerMessageDeflate2.extensionName];
        let opcode = options.binary ? 2 : 1;
        let rsv1 = options.compress;
        let byteLength;
        let readOnly;
        if (typeof data === "string") {
          byteLength = Buffer.byteLength(data);
          readOnly = false;
        } else if (isBlob(data)) {
          byteLength = data.size;
          readOnly = false;
        } else {
          data = toBuffer(data);
          byteLength = data.length;
          readOnly = toBuffer.readOnly;
        }
        if (this._firstFragment) {
          this._firstFragment = false;
          if (rsv1 && perMessageDeflate && perMessageDeflate.params[perMessageDeflate._isServer ? "server_no_context_takeover" : "client_no_context_takeover"]) {
            rsv1 = byteLength >= perMessageDeflate._threshold;
          }
          this._compress = rsv1;
        } else {
          rsv1 = false;
          opcode = 0;
        }
        if (options.fin) {this._firstFragment = true;}
        const opts = {
          [kByteLength]: byteLength,
          fin: options.fin,
          generateMask: this._generateMask,
          mask: options.mask,
          maskBuffer: this._maskBuffer,
          opcode,
          readOnly,
          rsv1
        };
        if (isBlob(data)) {
          if (this._state !== DEFAULT) {
            this.enqueue([this.getBlobData, data, this._compress, opts, cb]);
          } else {
            this.getBlobData(data, this._compress, opts, cb);
          }
        } else if (this._state !== DEFAULT) {
          this.enqueue([this.dispatch, data, this._compress, opts, cb]);
        } else {
          this.dispatch(data, this._compress, opts, cb);
        }
      }
      /**
       * Gets the contents of a blob as binary data.
       *
       * @param {Blob} blob The blob
       * @param {Boolean} [compress=false] Specifies whether or not to compress
       *     the data
       * @param {Object} options Options object
       * @param {Boolean} [options.fin=false] Specifies whether or not to set the
       *     FIN bit
       * @param {Function} [options.generateMask] The function used to generate the
       *     masking key
       * @param {Boolean} [options.mask=false] Specifies whether or not to mask
       *     `data`
       * @param {Buffer} [options.maskBuffer] The buffer used to store the masking
       *     key
       * @param {Number} options.opcode The opcode
       * @param {Boolean} [options.readOnly=false] Specifies whether `data` can be
       *     modified
       * @param {Boolean} [options.rsv1=false] Specifies whether or not to set the
       *     RSV1 bit
       * @param {Function} [cb] Callback
       * @private
       */
      getBlobData(blob, compress, options, cb) {
        this._bufferedBytes += options[kByteLength];
        this._state = GET_BLOB_DATA;
        blob.arrayBuffer().then((arrayBuffer) => {
          if (this._socket.destroyed) {
            const err = new Error(
              "The socket was closed while the blob was being read"
            );
            process.nextTick(callCallbacks, this, err, cb);
            return;
          }
          this._bufferedBytes -= options[kByteLength];
          const data = toBuffer(arrayBuffer);
          if (!compress) {
            this._state = DEFAULT;
            this.sendFrame(_Sender.frame(data, options), cb);
            this.dequeue();
          } else {
            this.dispatch(data, compress, options, cb);
          }
        }).catch((err) => {
          process.nextTick(onError, this, err, cb);
        });
      }
      /**
       * Dispatches a message.
       *
       * @param {(Buffer|String)} data The message to send
       * @param {Boolean} [compress=false] Specifies whether or not to compress
       *     `data`
       * @param {Object} options Options object
       * @param {Boolean} [options.fin=false] Specifies whether or not to set the
       *     FIN bit
       * @param {Function} [options.generateMask] The function used to generate the
       *     masking key
       * @param {Boolean} [options.mask=false] Specifies whether or not to mask
       *     `data`
       * @param {Buffer} [options.maskBuffer] The buffer used to store the masking
       *     key
       * @param {Number} options.opcode The opcode
       * @param {Boolean} [options.readOnly=false] Specifies whether `data` can be
       *     modified
       * @param {Boolean} [options.rsv1=false] Specifies whether or not to set the
       *     RSV1 bit
       * @param {Function} [cb] Callback
       * @private
       */
      dispatch(data, compress, options, cb) {
        if (!compress) {
          this.sendFrame(_Sender.frame(data, options), cb);
          return;
        }
        const perMessageDeflate = this._extensions[PerMessageDeflate2.extensionName];
        this._bufferedBytes += options[kByteLength];
        this._state = DEFLATING;
        perMessageDeflate.compress(data, options.fin, (_, buf) => {
          if (this._socket.destroyed) {
            const err = new Error(
              "The socket was closed while data was being compressed"
            );
            callCallbacks(this, err, cb);
            return;
          }
          this._bufferedBytes -= options[kByteLength];
          this._state = DEFAULT;
          options.readOnly = false;
          this.sendFrame(_Sender.frame(buf, options), cb);
          this.dequeue();
        });
      }
      /**
       * Executes queued send operations.
       *
       * @private
       */
      dequeue() {
        while (this._state === DEFAULT && this._queue.length) {
          const params = this._queue.shift();
          this._bufferedBytes -= params[3][kByteLength];
          Reflect.apply(params[0], this, params.slice(1));
        }
      }
      /**
       * Enqueues a send operation.
       *
       * @param {Array} params Send operation parameters.
       * @private
       */
      enqueue(params) {
        this._bufferedBytes += params[3][kByteLength];
        this._queue.push(params);
      }
      /**
       * Sends a frame.
       *
       * @param {(Buffer | String)[]} list The frame to send
       * @param {Function} [cb] Callback
       * @private
       */
      sendFrame(list, cb) {
        if (list.length === 2) {
          this._socket.cork();
          this._socket.write(list[0]);
          this._socket.write(list[1], cb);
          this._socket.uncork();
        } else {
          this._socket.write(list[0], cb);
        }
      }
    };
    module2.exports = Sender2;
    function callCallbacks(sender, err, cb) {
      if (typeof cb === "function") {cb(err);}
      for (let i = 0; i < sender._queue.length; i++) {
        const params = sender._queue[i];
        const callback = params.at(-1);
        if (typeof callback === "function") {callback(err);}
      }
    }
    function onError(sender, err, cb) {
      callCallbacks(sender, err, cb);
      sender.onerror(err);
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/event-target.js
var require_event_target = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/event-target.js"(exports2, module2) {
    "use strict";
    var { kForOnEventAttribute, kListener } = require_constants();
    var kCode = /* @__PURE__ */ Symbol("kCode");
    var kData = /* @__PURE__ */ Symbol("kData");
    var kError = /* @__PURE__ */ Symbol("kError");
    var kMessage = /* @__PURE__ */ Symbol("kMessage");
    var kReason = /* @__PURE__ */ Symbol("kReason");
    var kTarget = /* @__PURE__ */ Symbol("kTarget");
    var kType = /* @__PURE__ */ Symbol("kType");
    var kWasClean = /* @__PURE__ */ Symbol("kWasClean");
    var Event = class {
      /**
       * Create a new `Event`.
       *
       * @param {String} type The name of the event
       * @throws {TypeError} If the `type` argument is not specified
       */
      constructor(type) {
        this[kTarget] = null;
        this[kType] = type;
      }
      /**
       * @type {*}
       */
      get target() {
        return this[kTarget];
      }
      /**
       * @type {String}
       */
      get type() {
        return this[kType];
      }
    };
    Object.defineProperty(Event.prototype, "target", { enumerable: true });
    Object.defineProperty(Event.prototype, "type", { enumerable: true });
    var CloseEvent = class extends Event {
      /**
       * Create a new `CloseEvent`.
       *
       * @param {String} type The name of the event
       * @param {Object} [options] A dictionary object that allows for setting
       *     attributes via object members of the same name
       * @param {Number} [options.code=0] The status code explaining why the
       *     connection was closed
       * @param {String} [options.reason=''] A human-readable string explaining why
       *     the connection was closed
       * @param {Boolean} [options.wasClean=false] Indicates whether or not the
       *     connection was cleanly closed
       */
      constructor(type, options = {}) {
        super(type);
        this[kCode] = options.code === void 0 ? 0 : options.code;
        this[kReason] = options.reason === void 0 ? "" : options.reason;
        this[kWasClean] = options.wasClean === void 0 ? false : options.wasClean;
      }
      /**
       * @type {Number}
       */
      get code() {
        return this[kCode];
      }
      /**
       * @type {String}
       */
      get reason() {
        return this[kReason];
      }
      /**
       * @type {Boolean}
       */
      get wasClean() {
        return this[kWasClean];
      }
    };
    Object.defineProperty(CloseEvent.prototype, "code", { enumerable: true });
    Object.defineProperty(CloseEvent.prototype, "reason", { enumerable: true });
    Object.defineProperty(CloseEvent.prototype, "wasClean", { enumerable: true });
    var ErrorEvent = class extends Event {
      /**
       * Create a new `ErrorEvent`.
       *
       * @param {String} type The name of the event
       * @param {Object} [options] A dictionary object that allows for setting
       *     attributes via object members of the same name
       * @param {*} [options.error=null] The error that generated this event
       * @param {String} [options.message=''] The error message
       */
      constructor(type, options = {}) {
        super(type);
        this[kError] = options.error === void 0 ? null : options.error;
        this[kMessage] = options.message === void 0 ? "" : options.message;
      }
      /**
       * @type {*}
       */
      get error() {
        return this[kError];
      }
      /**
       * @type {String}
       */
      get message() {
        return this[kMessage];
      }
    };
    Object.defineProperty(ErrorEvent.prototype, "error", { enumerable: true });
    Object.defineProperty(ErrorEvent.prototype, "message", { enumerable: true });
    var MessageEvent = class extends Event {
      /**
       * Create a new `MessageEvent`.
       *
       * @param {String} type The name of the event
       * @param {Object} [options] A dictionary object that allows for setting
       *     attributes via object members of the same name
       * @param {*} [options.data=null] The message content
       */
      constructor(type, options = {}) {
        super(type);
        this[kData] = options.data === void 0 ? null : options.data;
      }
      /**
       * @type {*}
       */
      get data() {
        return this[kData];
      }
    };
    Object.defineProperty(MessageEvent.prototype, "data", { enumerable: true });
    var EventTarget = {
      /**
       * Register an event listener.
       *
       * @param {String} type A string representing the event type to listen for
       * @param {(Function|Object)} handler The listener to add
       * @param {Object} [options] An options object specifies characteristics about
       *     the event listener
       * @param {Boolean} [options.once=false] A `Boolean` indicating that the
       *     listener should be invoked at most once after being added. If `true`,
       *     the listener would be automatically removed when invoked.
       * @public
       */
      addEventListener(type, handler, options = {}) {
        for (const listener of this.listeners(type)) {
          if (!options[kForOnEventAttribute] && listener[kListener] === handler && !listener[kForOnEventAttribute]) {
            return;
          }
        }
        let wrapper;
        if (type === "message") {
          wrapper = function onMessage(data, isBinary) {
            const event = new MessageEvent("message", {
              data: isBinary ? data : data.toString()
            });
            event[kTarget] = this;
            callListener(handler, this, event);
          };
        } else if (type === "close") {
          wrapper = function onClose(code, message) {
            const event = new CloseEvent("close", {
              code,
              reason: message.toString(),
              wasClean: this._closeFrameReceived && this._closeFrameSent
            });
            event[kTarget] = this;
            callListener(handler, this, event);
          };
        } else if (type === "error") {
          wrapper = function onError(error) {
            const event = new ErrorEvent("error", {
              error,
              message: error.message
            });
            event[kTarget] = this;
            callListener(handler, this, event);
          };
        } else if (type === "open") {
          wrapper = function onOpen() {
            const event = new Event("open");
            event[kTarget] = this;
            callListener(handler, this, event);
          };
        } else {
          return;
        }
        wrapper[kForOnEventAttribute] = !!options[kForOnEventAttribute];
        wrapper[kListener] = handler;
        if (options.once) {
          this.once(type, wrapper);
        } else {
          this.on(type, wrapper);
        }
      },
      /**
       * Remove an event listener.
       *
       * @param {String} type A string representing the event type to remove
       * @param {(Function|Object)} handler The listener to remove
       * @public
       */
      removeEventListener(type, handler) {
        for (const listener of this.listeners(type)) {
          if (listener[kListener] === handler && !listener[kForOnEventAttribute]) {
            this.removeListener(type, listener);
            break;
          }
        }
      }
    };
    module2.exports = {
      CloseEvent,
      ErrorEvent,
      Event,
      EventTarget,
      MessageEvent
    };
    function callListener(listener, thisArg, event) {
      if (typeof listener === "object" && listener.handleEvent) {
        listener.handleEvent.call(listener, event);
      } else {
        listener.call(thisArg, event);
      }
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/extension.js
var require_extension = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/extension.js"(exports2, module2) {
    "use strict";
    var { tokenChars } = require_validation();
    function push(dest, name, elem) {
      if (dest[name] === void 0) {dest[name] = [elem];}
      else {dest[name].push(elem);}
    }
    function parse(header) {
      const offers = /* @__PURE__ */ Object.create(null);
      let params = /* @__PURE__ */ Object.create(null);
      let mustUnescape = false;
      let isEscaping = false;
      let inQuotes = false;
      let extensionName;
      let paramName;
      let start = -1;
      let code = -1;
      let end = -1;
      let i = 0;
      for (; i < header.length; i++) {
        code = header.charCodeAt(i);
        if (extensionName === void 0) {
          if (end === -1 && tokenChars[code] === 1) {
            if (start === -1) {start = i;}
          } else if (i !== 0 && (code === 32 || code === 9)) {
            if (end === -1 && start !== -1) {end = i;}
          } else if (code === 59 || code === 44) {
            if (start === -1) {
              throw new SyntaxError(`Unexpected character at index ${i}`);
            }
            if (end === -1) {end = i;}
            const name = header.slice(start, end);
            if (code === 44) {
              push(offers, name, params);
              params = /* @__PURE__ */ Object.create(null);
            } else {
              extensionName = name;
            }
            start = end = -1;
          } else {
            throw new SyntaxError(`Unexpected character at index ${i}`);
          }
        } else if (paramName === void 0) {
          if (end === -1 && tokenChars[code] === 1) {
            if (start === -1) {start = i;}
          } else if (code === 32 || code === 9) {
            if (end === -1 && start !== -1) {end = i;}
          } else if (code === 59 || code === 44) {
            if (start === -1) {
              throw new SyntaxError(`Unexpected character at index ${i}`);
            }
            if (end === -1) {end = i;}
            push(params, header.slice(start, end), true);
            if (code === 44) {
              push(offers, extensionName, params);
              params = /* @__PURE__ */ Object.create(null);
              extensionName = void 0;
            }
            start = end = -1;
          } else if (code === 61 && start !== -1 && end === -1) {
            paramName = header.slice(start, i);
            start = end = -1;
          } else {
            throw new SyntaxError(`Unexpected character at index ${i}`);
          }
        } else {
          if (isEscaping) {
            if (tokenChars[code] !== 1) {
              throw new SyntaxError(`Unexpected character at index ${i}`);
            }
            if (start === -1) {start = i;}
            else if (!mustUnescape) {mustUnescape = true;}
            isEscaping = false;
          } else if (inQuotes) {
            if (tokenChars[code] === 1) {
              if (start === -1) {start = i;}
            } else if (code === 34 && start !== -1) {
              inQuotes = false;
              end = i;
            } else if (code === 92) {
              isEscaping = true;
            } else {
              throw new SyntaxError(`Unexpected character at index ${i}`);
            }
          } else if (code === 34 && header.charCodeAt(i - 1) === 61) {
            inQuotes = true;
          } else if (end === -1 && tokenChars[code] === 1) {
            if (start === -1) {start = i;}
          } else if (start !== -1 && (code === 32 || code === 9)) {
            if (end === -1) {end = i;}
          } else if (code === 59 || code === 44) {
            if (start === -1) {
              throw new SyntaxError(`Unexpected character at index ${i}`);
            }
            if (end === -1) {end = i;}
            let value = header.slice(start, end);
            if (mustUnescape) {
              value = value.replace(/\\/g, "");
              mustUnescape = false;
            }
            push(params, paramName, value);
            if (code === 44) {
              push(offers, extensionName, params);
              params = /* @__PURE__ */ Object.create(null);
              extensionName = void 0;
            }
            paramName = void 0;
            start = end = -1;
          } else {
            throw new SyntaxError(`Unexpected character at index ${i}`);
          }
        }
      }
      if (start === -1 || inQuotes || code === 32 || code === 9) {
        throw new SyntaxError("Unexpected end of input");
      }
      if (end === -1) {end = i;}
      const token = header.slice(start, end);
      if (extensionName === void 0) {
        push(offers, token, params);
      } else {
        if (paramName === void 0) {
          push(params, token, true);
        } else if (mustUnescape) {
          push(params, paramName, token.replace(/\\/g, ""));
        } else {
          push(params, paramName, token);
        }
        push(offers, extensionName, params);
      }
      return offers;
    }
    function format(extensions) {
      return Object.keys(extensions).map((extension2) => {
        let configurations = extensions[extension2];
        if (!Array.isArray(configurations)) {configurations = [configurations];}
        return configurations.map((params) => {
          return [extension2].concat(
            Object.keys(params).map((k) => {
              let values = params[k];
              if (!Array.isArray(values)) {values = [values];}
              return values.map((v) => v === true ? k : `${k}=${v}`).join("; ");
            })
          ).join("; ");
        }).join(", ");
      }).join(", ");
    }
    module2.exports = { format, parse };
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/websocket.js
var require_websocket = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/websocket.js"(exports2, module2) {
    "use strict";
    var EventEmitter = require("node:events");
    var https = require("node:https");
    var http = require("node:http");
    var net3 = require("node:net");
    var tls = require("node:tls");
    var { randomBytes: randomBytes2, createHash } = require("node:crypto");
    var { Duplex, Readable } = require("node:stream");
    var { URL: URL2 } = require("node:url");
    var PerMessageDeflate2 = require_permessage_deflate();
    var Receiver2 = require_receiver();
    var Sender2 = require_sender();
    var { isBlob } = require_validation();
    var {
      BINARY_TYPES,
      CLOSE_TIMEOUT,
      EMPTY_BUFFER,
      GUID,
      kForOnEventAttribute,
      kListener,
      kStatusCode,
      kWebSocket,
      NOOP
    } = require_constants();
    var {
      EventTarget: { addEventListener, removeEventListener }
    } = require_event_target();
    var { format, parse } = require_extension();
    var { toBuffer } = require_buffer_util();
    var kAborted = /* @__PURE__ */ Symbol("kAborted");
    var protocolVersions = [8, 13];
    var readyStates = ["CONNECTING", "OPEN", "CLOSING", "CLOSED"];
    var subprotocolRegex = /^[!#$%&'*+\-.0-9A-Z^_`|a-z~]+$/;
    var WebSocket2 = class _WebSocket extends EventEmitter {
      /**
       * Create a new `WebSocket`.
       *
       * @param {(String|URL)} address The URL to which to connect
       * @param {(String|String[])} [protocols] The subprotocols
       * @param {Object} [options] Connection options
       */
      constructor(address, protocols, options) {
        super();
        this._binaryType = BINARY_TYPES[0];
        this._closeCode = 1006;
        this._closeFrameReceived = false;
        this._closeFrameSent = false;
        this._closeMessage = EMPTY_BUFFER;
        this._closeTimer = null;
        this._errorEmitted = false;
        this._extensions = {};
        this._paused = false;
        this._protocol = "";
        this._readyState = _WebSocket.CONNECTING;
        this._receiver = null;
        this._sender = null;
        this._socket = null;
        if (address !== null) {
          this._bufferedAmount = 0;
          this._isServer = false;
          this._redirects = 0;
          if (protocols === void 0) {
            protocols = [];
          } else if (!Array.isArray(protocols)) {
            if (typeof protocols === "object" && protocols !== null) {
              options = protocols;
              protocols = [];
            } else {
              protocols = [protocols];
            }
          }
          initAsClient(this, address, protocols, options);
        } else {
          this._autoPong = options.autoPong;
          this._closeTimeout = options.closeTimeout;
          this._isServer = true;
        }
      }
      /**
       * For historical reasons, the custom "nodebuffer" type is used by the default
       * instead of "blob".
       *
       * @type {String}
       */
      get binaryType() {
        return this._binaryType;
      }
      set binaryType(type) {
        if (!BINARY_TYPES.includes(type)) {return;}
        this._binaryType = type;
        if (this._receiver) {this._receiver._binaryType = type;}
      }
      /**
       * @type {Number}
       */
      get bufferedAmount() {
        if (!this._socket) {return this._bufferedAmount;}
        return this._socket._writableState.length + this._sender._bufferedBytes;
      }
      /**
       * @type {String}
       */
      get extensions() {
        return Object.keys(this._extensions).join();
      }
      /**
       * @type {Boolean}
       */
      get isPaused() {
        return this._paused;
      }
      /**
       * @type {Function}
       */
      /* istanbul ignore next */
      get onclose() {
        return null;
      }
      /**
       * @type {Function}
       */
      /* istanbul ignore next */
      get onerror() {
        return null;
      }
      /**
       * @type {Function}
       */
      /* istanbul ignore next */
      get onopen() {
        return null;
      }
      /**
       * @type {Function}
       */
      /* istanbul ignore next */
      get onmessage() {
        return null;
      }
      /**
       * @type {String}
       */
      get protocol() {
        return this._protocol;
      }
      /**
       * @type {Number}
       */
      get readyState() {
        return this._readyState;
      }
      /**
       * @type {String}
       */
      get url() {
        return this._url;
      }
      /**
       * Set up the socket and the internal resources.
       *
       * @param {Duplex} socket The network socket between the server and client
       * @param {Buffer} head The first packet of the upgraded stream
       * @param {Object} options Options object
       * @param {Boolean} [options.allowSynchronousEvents=false] Specifies whether
       *     any of the `'message'`, `'ping'`, and `'pong'` events can be emitted
       *     multiple times in the same tick
       * @param {Function} [options.generateMask] The function used to generate the
       *     masking key
       * @param {Number} [options.maxBufferedChunks=0] The maximum number of
       *     buffered data chunks
       * @param {Number} [options.maxFragments=0] The maximum number of message
       *     fragments
       * @param {Number} [options.maxPayload=0] The maximum allowed message size
       * @param {Boolean} [options.skipUTF8Validation=false] Specifies whether or
       *     not to skip UTF-8 validation for text and close messages
       * @private
       */
      setSocket(socket2, head, options) {
        const receiver = new Receiver2({
          allowSynchronousEvents: options.allowSynchronousEvents,
          binaryType: this.binaryType,
          extensions: this._extensions,
          isServer: this._isServer,
          maxBufferedChunks: options.maxBufferedChunks,
          maxFragments: options.maxFragments,
          maxPayload: options.maxPayload,
          skipUTF8Validation: options.skipUTF8Validation
        });
        const sender = new Sender2(socket2, this._extensions, options.generateMask);
        this._receiver = receiver;
        this._sender = sender;
        this._socket = socket2;
        receiver[kWebSocket] = this;
        sender[kWebSocket] = this;
        socket2[kWebSocket] = this;
        receiver.on("conclude", receiverOnConclude);
        receiver.on("drain", receiverOnDrain);
        receiver.on("error", receiverOnError);
        receiver.on("message", receiverOnMessage);
        receiver.on("ping", receiverOnPing);
        receiver.on("pong", receiverOnPong);
        sender.onerror = senderOnError;
        if (socket2.setTimeout) {socket2.setTimeout(0);}
        if (socket2.setNoDelay) {socket2.setNoDelay();}
        if (head.length > 0) {socket2.unshift(head);}
        socket2.on("close", socketOnClose);
        socket2.on("data", socketOnData);
        socket2.on("end", socketOnEnd);
        socket2.on("error", socketOnError);
        this._readyState = _WebSocket.OPEN;
        this.emit("open");
      }
      /**
       * Emit the `'close'` event.
       *
       * @private
       */
      emitClose() {
        if (!this._socket) {
          this._readyState = _WebSocket.CLOSED;
          this.emit("close", this._closeCode, this._closeMessage);
          return;
        }
        if (this._extensions[PerMessageDeflate2.extensionName]) {
          this._extensions[PerMessageDeflate2.extensionName].cleanup();
        }
        this._receiver.removeAllListeners();
        this._readyState = _WebSocket.CLOSED;
        this.emit("close", this._closeCode, this._closeMessage);
      }
      /**
       * Start a closing handshake.
       *
       *          +----------+   +-----------+   +----------+
       *     - - -|ws.close()|-->|close frame|-->|ws.close()|- - -
       *    |     +----------+   +-----------+   +----------+     |
       *          +----------+   +-----------+         |
       * CLOSING  |ws.close()|<--|close frame|<--+-----+       CLOSING
       *          +----------+   +-----------+   |
       *    |           |                        |   +---+        |
       *                +------------------------+-->|fin| - - - -
       *    |         +---+                      |   +---+
       *     - - - - -|fin|<---------------------+
       *              +---+
       *
       * @param {Number} [code] Status code explaining why the connection is closing
       * @param {(String|Buffer)} [data] The reason why the connection is
       *     closing
       * @public
       */
      close(code, data) {
        if (this.readyState === _WebSocket.CLOSED) {return;}
        if (this.readyState === _WebSocket.CONNECTING) {
          const msg = "WebSocket was closed before the connection was established";
          abortHandshake(this, this._req, msg);
          return;
        }
        if (this.readyState === _WebSocket.CLOSING) {
          if (this._closeFrameSent && (this._closeFrameReceived || this._receiver._writableState.errorEmitted)) {
            this._socket.end();
          }
          return;
        }
        this._readyState = _WebSocket.CLOSING;
        this._sender.close(code, data, !this._isServer, (err) => {
          if (err) {return;}
          this._closeFrameSent = true;
          if (this._closeFrameReceived || this._receiver._writableState.errorEmitted) {
            this._socket.end();
          }
        });
        setCloseTimer(this);
      }
      /**
       * Pause the socket.
       *
       * @public
       */
      pause() {
        if (this.readyState === _WebSocket.CONNECTING || this.readyState === _WebSocket.CLOSED) {
          return;
        }
        this._paused = true;
        this._socket.pause();
      }
      /**
       * Send a ping.
       *
       * @param {*} [data] The data to send
       * @param {Boolean} [mask] Indicates whether or not to mask `data`
       * @param {Function} [cb] Callback which is executed when the ping is sent
       * @public
       */
      ping(data, mask, cb) {
        if (this.readyState === _WebSocket.CONNECTING) {
          throw new Error("WebSocket is not open: readyState 0 (CONNECTING)");
        }
        if (typeof data === "function") {
          cb = data;
          data = mask = void 0;
        } else if (typeof mask === "function") {
          cb = mask;
          mask = void 0;
        }
        if (typeof data === "number") {data = data.toString();}
        if (this.readyState !== _WebSocket.OPEN) {
          sendAfterClose(this, data, cb);
          return;
        }
        if (mask === void 0) {mask = !this._isServer;}
        this._sender.ping(data || EMPTY_BUFFER, mask, cb);
      }
      /**
       * Send a pong.
       *
       * @param {*} [data] The data to send
       * @param {Boolean} [mask] Indicates whether or not to mask `data`
       * @param {Function} [cb] Callback which is executed when the pong is sent
       * @public
       */
      pong(data, mask, cb) {
        if (this.readyState === _WebSocket.CONNECTING) {
          throw new Error("WebSocket is not open: readyState 0 (CONNECTING)");
        }
        if (typeof data === "function") {
          cb = data;
          data = mask = void 0;
        } else if (typeof mask === "function") {
          cb = mask;
          mask = void 0;
        }
        if (typeof data === "number") {data = data.toString();}
        if (this.readyState !== _WebSocket.OPEN) {
          sendAfterClose(this, data, cb);
          return;
        }
        if (mask === void 0) {mask = !this._isServer;}
        this._sender.pong(data || EMPTY_BUFFER, mask, cb);
      }
      /**
       * Resume the socket.
       *
       * @public
       */
      resume() {
        if (this.readyState === _WebSocket.CONNECTING || this.readyState === _WebSocket.CLOSED) {
          return;
        }
        this._paused = false;
        if (!this._receiver._writableState.needDrain) {this._socket.resume();}
      }
      /**
       * Send a data message.
       *
       * @param {*} data The message to send
       * @param {Object} [options] Options object
       * @param {Boolean} [options.binary] Specifies whether `data` is binary or
       *     text
       * @param {Boolean} [options.compress] Specifies whether or not to compress
       *     `data`
       * @param {Boolean} [options.fin=true] Specifies whether the fragment is the
       *     last one
       * @param {Boolean} [options.mask] Specifies whether or not to mask `data`
       * @param {Function} [cb] Callback which is executed when data is written out
       * @public
       */
      send(data, options, cb) {
        if (this.readyState === _WebSocket.CONNECTING) {
          throw new Error("WebSocket is not open: readyState 0 (CONNECTING)");
        }
        if (typeof options === "function") {
          cb = options;
          options = {};
        }
        if (typeof data === "number") {data = data.toString();}
        if (this.readyState !== _WebSocket.OPEN) {
          sendAfterClose(this, data, cb);
          return;
        }
        const opts = {
          binary: typeof data !== "string",
          mask: !this._isServer,
          compress: true,
          fin: true,
          ...options
        };
        if (!this._extensions[PerMessageDeflate2.extensionName]) {
          opts.compress = false;
        }
        this._sender.send(data || EMPTY_BUFFER, opts, cb);
      }
      /**
       * Forcibly close the connection.
       *
       * @public
       */
      terminate() {
        if (this.readyState === _WebSocket.CLOSED) {return;}
        if (this.readyState === _WebSocket.CONNECTING) {
          const msg = "WebSocket was closed before the connection was established";
          abortHandshake(this, this._req, msg);
          return;
        }
        if (this._socket) {
          this._readyState = _WebSocket.CLOSING;
          this._socket.destroy();
        }
      }
    };
    Object.defineProperty(WebSocket2, "CONNECTING", {
      enumerable: true,
      value: readyStates.indexOf("CONNECTING")
    });
    Object.defineProperty(WebSocket2.prototype, "CONNECTING", {
      enumerable: true,
      value: readyStates.indexOf("CONNECTING")
    });
    Object.defineProperty(WebSocket2, "OPEN", {
      enumerable: true,
      value: readyStates.indexOf("OPEN")
    });
    Object.defineProperty(WebSocket2.prototype, "OPEN", {
      enumerable: true,
      value: readyStates.indexOf("OPEN")
    });
    Object.defineProperty(WebSocket2, "CLOSING", {
      enumerable: true,
      value: readyStates.indexOf("CLOSING")
    });
    Object.defineProperty(WebSocket2.prototype, "CLOSING", {
      enumerable: true,
      value: readyStates.indexOf("CLOSING")
    });
    Object.defineProperty(WebSocket2, "CLOSED", {
      enumerable: true,
      value: readyStates.indexOf("CLOSED")
    });
    Object.defineProperty(WebSocket2.prototype, "CLOSED", {
      enumerable: true,
      value: readyStates.indexOf("CLOSED")
    });
    [
      "binaryType",
      "bufferedAmount",
      "extensions",
      "isPaused",
      "protocol",
      "readyState",
      "url"
    ].forEach((property) => {
      Object.defineProperty(WebSocket2.prototype, property, { enumerable: true });
    });
    ["open", "error", "close", "message"].forEach((method) => {
      Object.defineProperty(WebSocket2.prototype, `on${method}`, {
        enumerable: true,
        get() {
          for (const listener of this.listeners(method)) {
            if (listener[kForOnEventAttribute]) {return listener[kListener];}
          }
          return null;
        },
        set(handler) {
          for (const listener of this.listeners(method)) {
            if (listener[kForOnEventAttribute]) {
              this.removeListener(method, listener);
              break;
            }
          }
          if (typeof handler !== "function") {return;}
          this.addEventListener(method, handler, {
            [kForOnEventAttribute]: true
          });
        }
      });
    });
    WebSocket2.prototype.addEventListener = addEventListener;
    WebSocket2.prototype.removeEventListener = removeEventListener;
    module2.exports = WebSocket2;
    function initAsClient(websocket, address, protocols, options) {
      const opts = {
        allowSynchronousEvents: true,
        autoPong: true,
        closeTimeout: CLOSE_TIMEOUT,
        protocolVersion: protocolVersions[1],
        maxBufferedChunks: 1024 * 1024,
        maxFragments: 128 * 1024,
        maxPayload: 100 * 1024 * 1024,
        skipUTF8Validation: false,
        perMessageDeflate: true,
        followRedirects: false,
        maxRedirects: 10,
        ...options,
        socketPath: void 0,
        hostname: void 0,
        protocol: void 0,
        timeout: void 0,
        method: "GET",
        host: void 0,
        path: void 0,
        port: void 0
      };
      websocket._autoPong = opts.autoPong;
      websocket._closeTimeout = opts.closeTimeout;
      if (!protocolVersions.includes(opts.protocolVersion)) {
        throw new RangeError(
          `Unsupported protocol version: ${opts.protocolVersion} (supported versions: ${protocolVersions.join(", ")})`
        );
      }
      let parsedUrl;
      if (address instanceof URL2) {
        parsedUrl = address;
      } else {
        try {
          parsedUrl = new URL2(address);
        } catch {
          throw new SyntaxError(`Invalid URL: ${address}`);
        }
      }
      if (parsedUrl.protocol === "http:") {
        parsedUrl.protocol = "ws:";
      } else if (parsedUrl.protocol === "https:") {
        parsedUrl.protocol = "wss:";
      }
      websocket._url = parsedUrl.href;
      const isSecure = parsedUrl.protocol === "wss:";
      const isIpcUrl = parsedUrl.protocol === "ws+unix:";
      let invalidUrlMessage;
      if (parsedUrl.protocol !== "ws:" && !isSecure && !isIpcUrl) {
        invalidUrlMessage = `The URL's protocol must be one of "ws:", "wss:", "http:", "https:", or "ws+unix:"`;
      } else if (isIpcUrl && !parsedUrl.pathname) {
        invalidUrlMessage = "The URL's pathname is empty";
      } else if (parsedUrl.hash) {
        invalidUrlMessage = "The URL contains a fragment identifier";
      }
      if (invalidUrlMessage) {
        const err = new SyntaxError(invalidUrlMessage);
        if (websocket._redirects === 0) {
          throw err;
        } else {
          emitErrorAndClose(websocket, err);
          return;
        }
      }
      const defaultPort = isSecure ? 443 : 80;
      const key = randomBytes2(16).toString("base64");
      const request = isSecure ? https.request : http.request;
      const protocolSet = /* @__PURE__ */ new Set();
      let perMessageDeflate;
      opts.createConnection = opts.createConnection || (isSecure ? tlsConnect : netConnect);
      opts.defaultPort = opts.defaultPort || defaultPort;
      opts.port = parsedUrl.port || defaultPort;
      opts.host = parsedUrl.hostname.startsWith("[") ? parsedUrl.hostname.slice(1, -1) : parsedUrl.hostname;
      opts.headers = {
        ...opts.headers,
        "Sec-WebSocket-Version": opts.protocolVersion,
        "Sec-WebSocket-Key": key,
        Connection: "Upgrade",
        Upgrade: "websocket"
      };
      opts.path = parsedUrl.pathname + parsedUrl.search;
      opts.timeout = opts.handshakeTimeout;
      if (opts.perMessageDeflate) {
        perMessageDeflate = new PerMessageDeflate2({
          ...opts.perMessageDeflate,
          isServer: false,
          maxPayload: opts.maxPayload
        });
        opts.headers["Sec-WebSocket-Extensions"] = format({
          [PerMessageDeflate2.extensionName]: perMessageDeflate.offer()
        });
      }
      if (protocols.length) {
        for (const protocol of protocols) {
          if (typeof protocol !== "string" || !subprotocolRegex.test(protocol) || protocolSet.has(protocol)) {
            throw new SyntaxError(
              "An invalid or duplicated subprotocol was specified"
            );
          }
          protocolSet.add(protocol);
        }
        opts.headers["Sec-WebSocket-Protocol"] = protocols.join(",");
      }
      if (opts.origin) {
        if (opts.protocolVersion < 13) {
          opts.headers["Sec-WebSocket-Origin"] = opts.origin;
        } else {
          opts.headers.Origin = opts.origin;
        }
      }
      if (parsedUrl.username || parsedUrl.password) {
        opts.auth = `${parsedUrl.username}:${parsedUrl.password}`;
      }
      if (isIpcUrl) {
        const parts = opts.path.split(":");
        opts.socketPath = parts[0];
        opts.path = parts[1];
      }
      let req;
      if (opts.followRedirects) {
        if (websocket._redirects === 0) {
          websocket._originalIpc = isIpcUrl;
          websocket._originalSecure = isSecure;
          websocket._originalHostOrSocketPath = isIpcUrl ? opts.socketPath : parsedUrl.host;
          const headers = options && options.headers;
          options = { ...options, headers: {} };
          if (headers) {
            for (const [key2, value] of Object.entries(headers)) {
              options.headers[key2.toLowerCase()] = value;
            }
          }
        } else if (websocket.listenerCount("redirect") === 0) {
          const isSameHost = isIpcUrl ? websocket._originalIpc ? opts.socketPath === websocket._originalHostOrSocketPath : false : websocket._originalIpc ? false : parsedUrl.host === websocket._originalHostOrSocketPath;
          if (!isSameHost || websocket._originalSecure && !isSecure) {
            delete opts.headers.authorization;
            delete opts.headers.cookie;
            if (!isSameHost) {delete opts.headers.host;}
            opts.auth = void 0;
          }
        }
        if (opts.auth && !options.headers.authorization) {
          options.headers.authorization = `Basic ${  Buffer.from(opts.auth).toString("base64")}`;
        }
        req = websocket._req = request(opts);
        if (websocket._redirects) {
          websocket.emit("redirect", websocket.url, req);
        }
      } else {
        req = websocket._req = request(opts);
      }
      if (opts.timeout) {
        req.on("timeout", () => {
          abortHandshake(websocket, req, "Opening handshake has timed out");
        });
      }
      req.on("error", (err) => {
        if (req === null || req[kAborted]) {return;}
        req = websocket._req = null;
        emitErrorAndClose(websocket, err);
      });
      req.on("response", (res) => {
        const location = res.headers.location;
        const statusCode = res.statusCode;
        if (location && opts.followRedirects && statusCode >= 300 && statusCode < 400) {
          if (++websocket._redirects > opts.maxRedirects) {
            abortHandshake(websocket, req, "Maximum redirects exceeded");
            return;
          }
          req.abort();
          let addr;
          try {
            addr = new URL2(location, address);
          } catch (e) {
            const err = new SyntaxError(`Invalid URL: ${location}`);
            emitErrorAndClose(websocket, err);
            return;
          }
          initAsClient(websocket, addr, protocols, options);
        } else if (!websocket.emit("unexpected-response", req, res)) {
          abortHandshake(
            websocket,
            req,
            `Unexpected server response: ${res.statusCode}`
          );
        }
      });
      req.on("upgrade", (res, socket2, head) => {
        websocket.emit("upgrade", res);
        if (websocket.readyState !== WebSocket2.CONNECTING) {return;}
        req = websocket._req = null;
        const upgrade = res.headers.upgrade;
        if (upgrade === void 0 || upgrade.toLowerCase() !== "websocket") {
          abortHandshake(websocket, socket2, "Invalid Upgrade header");
          return;
        }
        const digest = createHash("sha1").update(key + GUID).digest("base64");
        if (res.headers["sec-websocket-accept"] !== digest) {
          abortHandshake(websocket, socket2, "Invalid Sec-WebSocket-Accept header");
          return;
        }
        const serverProt = res.headers["sec-websocket-protocol"];
        let protError;
        if (serverProt !== void 0) {
          if (!protocolSet.size) {
            protError = "Server sent a subprotocol but none was requested";
          } else if (!protocolSet.has(serverProt)) {
            protError = "Server sent an invalid subprotocol";
          }
        } else if (protocolSet.size) {
          protError = "Server sent no subprotocol";
        }
        if (protError) {
          abortHandshake(websocket, socket2, protError);
          return;
        }
        if (serverProt) {websocket._protocol = serverProt;}
        const secWebSocketExtensions = res.headers["sec-websocket-extensions"];
        if (secWebSocketExtensions !== void 0) {
          if (!perMessageDeflate) {
            const message = "Server sent a Sec-WebSocket-Extensions header but no extension was requested";
            abortHandshake(websocket, socket2, message);
            return;
          }
          let extensions;
          try {
            extensions = parse(secWebSocketExtensions);
          } catch (err) {
            const message = "Invalid Sec-WebSocket-Extensions header";
            abortHandshake(websocket, socket2, message);
            return;
          }
          const extensionNames = Object.keys(extensions);
          if (extensionNames.length !== 1 || extensionNames[0] !== PerMessageDeflate2.extensionName) {
            const message = "Server indicated an extension that was not requested";
            abortHandshake(websocket, socket2, message);
            return;
          }
          try {
            perMessageDeflate.accept(extensions[PerMessageDeflate2.extensionName]);
          } catch (err) {
            const message = "Invalid Sec-WebSocket-Extensions header";
            abortHandshake(websocket, socket2, message);
            return;
          }
          websocket._extensions[PerMessageDeflate2.extensionName] = perMessageDeflate;
        }
        websocket.setSocket(socket2, head, {
          allowSynchronousEvents: opts.allowSynchronousEvents,
          generateMask: opts.generateMask,
          maxBufferedChunks: opts.maxBufferedChunks,
          maxFragments: opts.maxFragments,
          maxPayload: opts.maxPayload,
          skipUTF8Validation: opts.skipUTF8Validation
        });
      });
      if (opts.finishRequest) {
        opts.finishRequest(req, websocket);
      } else {
        req.end();
      }
    }
    function emitErrorAndClose(websocket, err) {
      websocket._readyState = WebSocket2.CLOSING;
      websocket._errorEmitted = true;
      websocket.emit("error", err);
      websocket.emitClose();
    }
    function netConnect(options) {
      options.path = options.socketPath;
      return net3.connect(options);
    }
    function tlsConnect(options) {
      options.path = void 0;
      if (!options.servername && options.servername !== "") {
        options.servername = net3.isIP(options.host) ? "" : options.host;
      }
      return tls.connect(options);
    }
    function abortHandshake(websocket, stream, message) {
      websocket._readyState = WebSocket2.CLOSING;
      const err = new Error(message);
      Error.captureStackTrace(err, abortHandshake);
      if (stream.setHeader) {
        stream[kAborted] = true;
        stream.abort();
        if (stream.socket && !stream.socket.destroyed) {
          stream.socket.destroy();
        }
        process.nextTick(emitErrorAndClose, websocket, err);
      } else {
        stream.destroy(err);
        stream.once("error", websocket.emit.bind(websocket, "error"));
        stream.once("close", websocket.emitClose.bind(websocket));
      }
    }
    function sendAfterClose(websocket, data, cb) {
      if (data) {
        const length = isBlob(data) ? data.size : toBuffer(data).length;
        if (websocket._socket) {websocket._sender._bufferedBytes += length;}
        else {websocket._bufferedAmount += length;}
      }
      if (cb) {
        const err = new Error(
          `WebSocket is not open: readyState ${websocket.readyState} (${readyStates[websocket.readyState]})`
        );
        process.nextTick(cb, err);
      }
    }
    function receiverOnConclude(code, reason) {
      const websocket = this[kWebSocket];
      websocket._closeFrameReceived = true;
      websocket._closeMessage = reason;
      websocket._closeCode = code;
      if (websocket._socket[kWebSocket] === void 0) {return;}
      websocket._socket.removeListener("data", socketOnData);
      process.nextTick(resume, websocket._socket);
      if (code === 1005) {websocket.close();}
      else {websocket.close(code, reason);}
    }
    function receiverOnDrain() {
      const websocket = this[kWebSocket];
      if (!websocket.isPaused) {websocket._socket.resume();}
    }
    function receiverOnError(err) {
      const websocket = this[kWebSocket];
      if (websocket._socket[kWebSocket] !== void 0) {
        websocket._socket.removeListener("data", socketOnData);
        process.nextTick(resume, websocket._socket);
        websocket.close(err[kStatusCode]);
      }
      if (!websocket._errorEmitted) {
        websocket._errorEmitted = true;
        websocket.emit("error", err);
      }
    }
    function receiverOnFinish() {
      this[kWebSocket].emitClose();
    }
    function receiverOnMessage(data, isBinary) {
      this[kWebSocket].emit("message", data, isBinary);
    }
    function receiverOnPing(data) {
      const websocket = this[kWebSocket];
      if (websocket._autoPong) {websocket.pong(data, !this._isServer, NOOP);}
      websocket.emit("ping", data);
    }
    function receiverOnPong(data) {
      this[kWebSocket].emit("pong", data);
    }
    function resume(stream) {
      stream.resume();
    }
    function senderOnError(err) {
      const websocket = this[kWebSocket];
      if (websocket.readyState === WebSocket2.CLOSED) {return;}
      if (websocket.readyState === WebSocket2.OPEN) {
        websocket._readyState = WebSocket2.CLOSING;
        setCloseTimer(websocket);
      }
      this._socket.end();
      if (!websocket._errorEmitted) {
        websocket._errorEmitted = true;
        websocket.emit("error", err);
      }
    }
    function setCloseTimer(websocket) {
      websocket._closeTimer = setTimeout(
        websocket._socket.destroy.bind(websocket._socket),
        websocket._closeTimeout
      );
    }
    function socketOnClose() {
      const websocket = this[kWebSocket];
      this.removeListener("close", socketOnClose);
      this.removeListener("data", socketOnData);
      this.removeListener("end", socketOnEnd);
      websocket._readyState = WebSocket2.CLOSING;
      if (!this._readableState.endEmitted && !websocket._closeFrameReceived && !websocket._receiver._writableState.errorEmitted && this._readableState.length !== 0) {
        const chunk = this.read(this._readableState.length);
        websocket._receiver.write(chunk);
      }
      websocket._receiver.end();
      this[kWebSocket] = void 0;
      clearTimeout(websocket._closeTimer);
      if (websocket._receiver._writableState.finished || websocket._receiver._writableState.errorEmitted) {
        websocket.emitClose();
      } else {
        websocket._receiver.on("error", receiverOnFinish);
        websocket._receiver.on("finish", receiverOnFinish);
      }
    }
    function socketOnData(chunk) {
      if (!this[kWebSocket]._receiver.write(chunk)) {
        this.pause();
      }
    }
    function socketOnEnd() {
      const websocket = this[kWebSocket];
      websocket._readyState = WebSocket2.CLOSING;
      websocket._receiver.end();
      this.end();
    }
    function socketOnError() {
      const websocket = this[kWebSocket];
      this.removeListener("error", socketOnError);
      this.on("error", NOOP);
      if (websocket) {
        websocket._readyState = WebSocket2.CLOSING;
        this.destroy();
      }
    }
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/stream.js
var require_stream = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/stream.js"(exports2, module2) {
    "use strict";
    var WebSocket2 = require_websocket();
    var { Duplex } = require("node:stream");
    function emitClose(stream) {
      stream.emit("close");
    }
    function duplexOnEnd() {
      if (!this.destroyed && this._writableState.finished) {
        this.destroy();
      }
    }
    function duplexOnError(err) {
      this.removeListener("error", duplexOnError);
      this.destroy();
      if (this.listenerCount("error") === 0) {
        this.emit("error", err);
      }
    }
    function createWebSocketStream2(ws, options) {
      let terminateOnDestroy = true;
      const duplex = new Duplex({
        ...options,
        autoDestroy: false,
        emitClose: false,
        objectMode: false,
        writableObjectMode: false
      });
      ws.on("message", function message(msg, isBinary) {
        const data = !isBinary && duplex._readableState.objectMode ? msg.toString() : msg;
        if (!duplex.push(data)) {ws.pause();}
      });
      ws.once("error", function error(err) {
        if (duplex.destroyed) {return;}
        terminateOnDestroy = false;
        duplex.destroy(err);
      });
      ws.once("close", function close() {
        if (duplex.destroyed) {return;}
        duplex.push(null);
      });
      duplex._destroy = function(err, callback) {
        if (ws.readyState === ws.CLOSED) {
          callback(err);
          process.nextTick(emitClose, duplex);
          return;
        }
        let called = false;
        ws.once("error", function error(err2) {
          called = true;
          callback(err2);
        });
        ws.once("close", function close() {
          if (!called) {callback(err);}
          process.nextTick(emitClose, duplex);
        });
        if (terminateOnDestroy) {ws.terminate();}
      };
      duplex._final = function(callback) {
        if (ws.readyState === ws.CONNECTING) {
          ws.once("open", function open3() {
            duplex._final(callback);
          });
          return;
        }
        if (ws._socket === null) {return;}
        if (ws._socket._writableState.finished) {
          callback();
          if (duplex._readableState.endEmitted) {duplex.destroy();}
        } else {
          ws._socket.once("finish", function finish() {
            callback();
          });
          ws.close();
        }
      };
      duplex._read = function() {
        if (ws.isPaused) {ws.resume();}
      };
      duplex._write = function(chunk, encoding, callback) {
        if (ws.readyState === ws.CONNECTING) {
          ws.once("open", function open3() {
            duplex._write(chunk, encoding, callback);
          });
          return;
        }
        ws.send(chunk, callback);
      };
      duplex.on("end", duplexOnEnd);
      duplex.on("error", duplexOnError);
      return duplex;
    }
    module2.exports = createWebSocketStream2;
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/subprotocol.js
var require_subprotocol = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/subprotocol.js"(exports2, module2) {
    "use strict";
    var { tokenChars } = require_validation();
    function parse(header) {
      const protocols = /* @__PURE__ */ new Set();
      let start = -1;
      let end = -1;
      let i = 0;
      for (i; i < header.length; i++) {
        const code = header.charCodeAt(i);
        if (end === -1 && tokenChars[code] === 1) {
          if (start === -1) {start = i;}
        } else if (i !== 0 && (code === 32 || code === 9)) {
          if (end === -1 && start !== -1) {end = i;}
        } else if (code === 44) {
          if (start === -1) {
            throw new SyntaxError(`Unexpected character at index ${i}`);
          }
          if (end === -1) {end = i;}
          const protocol2 = header.slice(start, end);
          if (protocols.has(protocol2)) {
            throw new SyntaxError(`The "${protocol2}" subprotocol is duplicated`);
          }
          protocols.add(protocol2);
          start = end = -1;
        } else {
          throw new SyntaxError(`Unexpected character at index ${i}`);
        }
      }
      if (start === -1 || end !== -1) {
        throw new SyntaxError("Unexpected end of input");
      }
      const protocol = header.slice(start, i);
      if (protocols.has(protocol)) {
        throw new SyntaxError(`The "${protocol}" subprotocol is duplicated`);
      }
      protocols.add(protocol);
      return protocols;
    }
    module2.exports = { parse };
  }
});

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/websocket-server.js
var require_websocket_server = __commonJS({
  "../node_modules/.pnpm/ws@8.21.0/node_modules/ws/lib/websocket-server.js"(exports2, module2) {
    "use strict";
    var EventEmitter = require("node:events");
    var http = require("node:http");
    var { Duplex } = require("node:stream");
    var { createHash } = require("node:crypto");
    var extension2 = require_extension();
    var PerMessageDeflate2 = require_permessage_deflate();
    var subprotocol2 = require_subprotocol();
    var WebSocket2 = require_websocket();
    var { CLOSE_TIMEOUT, GUID, kWebSocket } = require_constants();
    var keyRegex = /^[+/0-9A-Za-z]{22}==$/;
    var RUNNING = 0;
    var CLOSING = 1;
    var CLOSED = 2;
    var WebSocketServer2 = class extends EventEmitter {
      /**
       * Create a `WebSocketServer` instance.
       *
       * @param {Object} options Configuration options
       * @param {Boolean} [options.allowSynchronousEvents=true] Specifies whether
       *     any of the `'message'`, `'ping'`, and `'pong'` events can be emitted
       *     multiple times in the same tick
       * @param {Boolean} [options.autoPong=true] Specifies whether or not to
       *     automatically send a pong in response to a ping
       * @param {Number} [options.backlog=511] The maximum length of the queue of
       *     pending connections
       * @param {Boolean} [options.clientTracking=true] Specifies whether or not to
       *     track clients
       * @param {Number} [options.closeTimeout=30000] Duration in milliseconds to
       *     wait for the closing handshake to finish after `websocket.close()` is
       *     called
       * @param {Function} [options.handleProtocols] A hook to handle protocols
       * @param {String} [options.host] The hostname where to bind the server
       * @param {Number} [options.maxBufferedChunks=1048576] The maximum number of
       *     buffered data chunks
       * @param {Number} [options.maxFragments=131072] The maximum number of message
       *     fragments
       * @param {Number} [options.maxPayload=104857600] The maximum allowed message
       *     size
       * @param {Boolean} [options.noServer=false] Enable no server mode
       * @param {String} [options.path] Accept only connections matching this path
       * @param {(Boolean|Object)} [options.perMessageDeflate=false] Enable/disable
       *     permessage-deflate
       * @param {Number} [options.port] The port where to bind the server
       * @param {(http.Server|https.Server)} [options.server] A pre-created HTTP/S
       *     server to use
       * @param {Boolean} [options.skipUTF8Validation=false] Specifies whether or
       *     not to skip UTF-8 validation for text and close messages
       * @param {Function} [options.verifyClient] A hook to reject connections
       * @param {Function} [options.WebSocket=WebSocket] Specifies the `WebSocket`
       *     class to use. It must be the `WebSocket` class or class that extends it
       * @param {Function} [callback] A listener for the `listening` event
       */
      constructor(options, callback) {
        super();
        options = {
          allowSynchronousEvents: true,
          autoPong: true,
          maxBufferedChunks: 1024 * 1024,
          maxFragments: 128 * 1024,
          maxPayload: 100 * 1024 * 1024,
          skipUTF8Validation: false,
          perMessageDeflate: false,
          handleProtocols: null,
          clientTracking: true,
          closeTimeout: CLOSE_TIMEOUT,
          verifyClient: null,
          noServer: false,
          backlog: null,
          // use default (511 as implemented in net.js)
          server: null,
          host: null,
          path: null,
          port: null,
          WebSocket: WebSocket2,
          ...options
        };
        if (options.port == null && !options.server && !options.noServer || options.port != null && (options.server || options.noServer) || options.server && options.noServer) {
          throw new TypeError(
            'One and only one of the "port", "server", or "noServer" options must be specified'
          );
        }
        if (options.port != null) {
          this._server = http.createServer((req, res) => {
            const body = http.STATUS_CODES[426];
            res.writeHead(426, {
              "Content-Length": body.length,
              "Content-Type": "text/plain"
            });
            res.end(body);
          });
          this._server.listen(
            options.port,
            options.host,
            options.backlog,
            callback
          );
        } else if (options.server) {
          this._server = options.server;
        }
        if (this._server) {
          const emitConnection = this.emit.bind(this, "connection");
          this._removeListeners = addListeners(this._server, {
            listening: this.emit.bind(this, "listening"),
            error: this.emit.bind(this, "error"),
            upgrade: (req, socket2, head) => {
              this.handleUpgrade(req, socket2, head, emitConnection);
            }
          });
        }
        if (options.perMessageDeflate === true) {options.perMessageDeflate = {};}
        if (options.clientTracking) {
          this.clients = /* @__PURE__ */ new Set();
          this._shouldEmitClose = false;
        }
        this.options = options;
        this._state = RUNNING;
      }
      /**
       * Returns the bound address, the address family name, and port of the server
       * as reported by the operating system if listening on an IP socket.
       * If the server is listening on a pipe or UNIX domain socket, the name is
       * returned as a string.
       *
       * @return {(Object|String|null)} The address of the server
       * @public
       */
      address() {
        if (this.options.noServer) {
          throw new Error('The server is operating in "noServer" mode');
        }
        if (!this._server) {return null;}
        return this._server.address();
      }
      /**
       * Stop the server from accepting new connections and emit the `'close'` event
       * when all existing connections are closed.
       *
       * @param {Function} [cb] A one-time listener for the `'close'` event
       * @public
       */
      close(cb) {
        if (this._state === CLOSED) {
          if (cb) {
            this.once("close", () => {
              cb(new Error("The server is not running"));
            });
          }
          process.nextTick(emitClose, this);
          return;
        }
        if (cb) {this.once("close", cb);}
        if (this._state === CLOSING) {return;}
        this._state = CLOSING;
        if (this.options.noServer || this.options.server) {
          if (this._server) {
            this._removeListeners();
            this._removeListeners = this._server = null;
          }
          if (this.clients) {
            if (!this.clients.size) {
              process.nextTick(emitClose, this);
            } else {
              this._shouldEmitClose = true;
            }
          } else {
            process.nextTick(emitClose, this);
          }
        } else {
          const server = this._server;
          this._removeListeners();
          this._removeListeners = this._server = null;
          server.close(() => {
            emitClose(this);
          });
        }
      }
      /**
       * See if a given request should be handled by this server instance.
       *
       * @param {http.IncomingMessage} req Request object to inspect
       * @return {Boolean} `true` if the request is valid, else `false`
       * @public
       */
      shouldHandle(req) {
        if (this.options.path) {
          const index = req.url.indexOf("?");
          const pathname = index !== -1 ? req.url.slice(0, index) : req.url;
          if (pathname !== this.options.path) {return false;}
        }
        return true;
      }
      /**
       * Handle a HTTP Upgrade request.
       *
       * @param {http.IncomingMessage} req The request object
       * @param {Duplex} socket The network socket between the server and client
       * @param {Buffer} head The first packet of the upgraded stream
       * @param {Function} cb Callback
       * @public
       */
      handleUpgrade(req, socket2, head, cb) {
        socket2.on("error", socketOnError);
        const key = req.headers["sec-websocket-key"];
        const upgrade = req.headers.upgrade;
        const version = +req.headers["sec-websocket-version"];
        if (req.method !== "GET") {
          const message = "Invalid HTTP method";
          abortHandshakeOrEmitwsClientError(this, req, socket2, 405, message);
          return;
        }
        if (upgrade === void 0 || upgrade.toLowerCase() !== "websocket") {
          const message = "Invalid Upgrade header";
          abortHandshakeOrEmitwsClientError(this, req, socket2, 400, message);
          return;
        }
        if (key === void 0 || !keyRegex.test(key)) {
          const message = "Missing or invalid Sec-WebSocket-Key header";
          abortHandshakeOrEmitwsClientError(this, req, socket2, 400, message);
          return;
        }
        if (version !== 13 && version !== 8) {
          const message = "Missing or invalid Sec-WebSocket-Version header";
          abortHandshakeOrEmitwsClientError(this, req, socket2, 400, message, {
            "Sec-WebSocket-Version": "13, 8"
          });
          return;
        }
        if (!this.shouldHandle(req)) {
          abortHandshake(socket2, 400);
          return;
        }
        const secWebSocketProtocol = req.headers["sec-websocket-protocol"];
        let protocols = /* @__PURE__ */ new Set();
        if (secWebSocketProtocol !== void 0) {
          try {
            protocols = subprotocol2.parse(secWebSocketProtocol);
          } catch (err) {
            const message = "Invalid Sec-WebSocket-Protocol header";
            abortHandshakeOrEmitwsClientError(this, req, socket2, 400, message);
            return;
          }
        }
        const secWebSocketExtensions = req.headers["sec-websocket-extensions"];
        const extensions = {};
        if (this.options.perMessageDeflate && secWebSocketExtensions !== void 0) {
          const perMessageDeflate = new PerMessageDeflate2({
            ...this.options.perMessageDeflate,
            isServer: true,
            maxPayload: this.options.maxPayload
          });
          try {
            const offers = extension2.parse(secWebSocketExtensions);
            if (offers[PerMessageDeflate2.extensionName]) {
              perMessageDeflate.accept(offers[PerMessageDeflate2.extensionName]);
              extensions[PerMessageDeflate2.extensionName] = perMessageDeflate;
            }
          } catch (err) {
            const message = "Invalid or unacceptable Sec-WebSocket-Extensions header";
            abortHandshakeOrEmitwsClientError(this, req, socket2, 400, message);
            return;
          }
        }
        if (this.options.verifyClient) {
          const info = {
            origin: req.headers[`${version === 8 ? "sec-websocket-origin" : "origin"}`],
            secure: !!(req.socket.authorized || req.socket.encrypted),
            req
          };
          if (this.options.verifyClient.length === 2) {
            this.options.verifyClient(info, (verified, code, message, headers) => {
              if (!verified) {
                return abortHandshake(socket2, code || 401, message, headers);
              }
              this.completeUpgrade(
                extensions,
                key,
                protocols,
                req,
                socket2,
                head,
                cb
              );
            });
            return;
          }
          if (!this.options.verifyClient(info)) {return abortHandshake(socket2, 401);}
        }
        this.completeUpgrade(extensions, key, protocols, req, socket2, head, cb);
      }
      /**
       * Upgrade the connection to WebSocket.
       *
       * @param {Object} extensions The accepted extensions
       * @param {String} key The value of the `Sec-WebSocket-Key` header
       * @param {Set} protocols The subprotocols
       * @param {http.IncomingMessage} req The request object
       * @param {Duplex} socket The network socket between the server and client
       * @param {Buffer} head The first packet of the upgraded stream
       * @param {Function} cb Callback
       * @throws {Error} If called more than once with the same socket
       * @private
       */
      completeUpgrade(extensions, key, protocols, req, socket2, head, cb) {
        if (!socket2.readable || !socket2.writable) {return socket2.destroy();}
        if (socket2[kWebSocket]) {
          throw new Error(
            "server.handleUpgrade() was called more than once with the same socket, possibly due to a misconfiguration"
          );
        }
        if (this._state > RUNNING) {return abortHandshake(socket2, 503);}
        const digest = createHash("sha1").update(key + GUID).digest("base64");
        const headers = [
          "HTTP/1.1 101 Switching Protocols",
          "Upgrade: websocket",
          "Connection: Upgrade",
          `Sec-WebSocket-Accept: ${digest}`
        ];
        const ws = new this.options.WebSocket(null, void 0, this.options);
        if (protocols.size) {
          const protocol = this.options.handleProtocols ? this.options.handleProtocols(protocols, req) : protocols.values().next().value;
          if (protocol) {
            headers.push(`Sec-WebSocket-Protocol: ${protocol}`);
            ws._protocol = protocol;
          }
        }
        if (extensions[PerMessageDeflate2.extensionName]) {
          const params = extensions[PerMessageDeflate2.extensionName].params;
          const value = extension2.format({
            [PerMessageDeflate2.extensionName]: [params]
          });
          headers.push(`Sec-WebSocket-Extensions: ${value}`);
          ws._extensions = extensions;
        }
        this.emit("headers", headers, req);
        socket2.write(headers.concat("\r\n").join("\r\n"));
        socket2.removeListener("error", socketOnError);
        ws.setSocket(socket2, head, {
          allowSynchronousEvents: this.options.allowSynchronousEvents,
          maxBufferedChunks: this.options.maxBufferedChunks,
          maxFragments: this.options.maxFragments,
          maxPayload: this.options.maxPayload,
          skipUTF8Validation: this.options.skipUTF8Validation
        });
        if (this.clients) {
          this.clients.add(ws);
          ws.on("close", () => {
            this.clients.delete(ws);
            if (this._shouldEmitClose && !this.clients.size) {
              process.nextTick(emitClose, this);
            }
          });
        }
        cb(ws, req);
      }
    };
    module2.exports = WebSocketServer2;
    function addListeners(server, map) {
      for (const event of Object.keys(map)) {server.on(event, map[event]);}
      return function removeListeners() {
        for (const event of Object.keys(map)) {
          server.removeListener(event, map[event]);
        }
      };
    }
    function emitClose(server) {
      server._state = CLOSED;
      server.emit("close");
    }
    function socketOnError() {
      this.destroy();
    }
    function abortHandshake(socket2, code, message, headers) {
      message = message || http.STATUS_CODES[code];
      headers = {
        Connection: "close",
        "Content-Type": "text/html",
        "Content-Length": Buffer.byteLength(message),
        ...headers
      };
      socket2.once("finish", socket2.destroy);
      socket2.end(
        `HTTP/1.1 ${code} ${http.STATUS_CODES[code]}\r
${  Object.keys(headers).map((h) => `${h}: ${headers[h]}`).join("\r\n")  }\r\n\r\n${  message}`
      );
    }
    function abortHandshakeOrEmitwsClientError(server, req, socket2, code, message, headers) {
      if (server.listenerCount("wsClientError")) {
        const err = new Error(message);
        Error.captureStackTrace(err, abortHandshakeOrEmitwsClientError);
        server.emit("wsClientError", err, socket2, req);
      } else {
        abortHandshake(socket2, code, message, headers);
      }
    }
  }
});

// src/shared/ssh-types.ts
var MAX_SSH_RELAY_GRACE_PERIOD_SECONDS, LEGACY_DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS, DEFAULT_BOUNDED_SSH_RELAY_GRACE_PERIOD_SECONDS, DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS;
var init_ssh_types = __esm({
  "src/shared/ssh-types.ts"() {
    "use strict";
    MAX_SSH_RELAY_GRACE_PERIOD_SECONDS = 7 * 24 * 60 * 60;
    LEGACY_DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS = 3 * 60 * 60;
    DEFAULT_BOUNDED_SSH_RELAY_GRACE_PERIOD_SECONDS = 24 * 60 * 60;
    DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS = 0;
  }
});

// src/main/ssh/relay-protocol.ts
var RELAY_VERSION, RELAY_SENTINEL, MAX_MESSAGE_SIZE, MessageType, DEFAULT_GRACE_TIME_MS, STREAM_CHUNK_SIZE, GIT_RESPONSE_STREAM_THRESHOLD, GIT_RESPONSE_CHUNK_SIZE;
var init_relay_protocol = __esm({
  "src/main/ssh/relay-protocol.ts"() {
    "use strict";
    init_ssh_types();
    RELAY_VERSION = "0.1.0";
    RELAY_SENTINEL = `ORCA-RELAY v${RELAY_VERSION} READY
`;
    MAX_MESSAGE_SIZE = 16 * 1024 * 1024;
    MessageType = {
      Regular: 1,
      KeepAlive: 9
    };
    DEFAULT_GRACE_TIME_MS = DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS * 1e3;
    STREAM_CHUNK_SIZE = 256 * 1024;
    GIT_RESPONSE_STREAM_THRESHOLD = 256 * 1024;
    GIT_RESPONSE_CHUNK_SIZE = 128 * 1024;
  }
});

// src/relay/agent-wire.ts
function createWireState() {
  return { seqCounter: 0, highestAck: 0 };
}
function encodeDataFrame(state, payload) {
  return encodeFrame(state, MessageType.Regular, payload);
}
function encodeKeepaliveFrame(state) {
  return encodeFrame(state, MessageType.KeepAlive, Buffer.alloc(0));
}
function encodeFrame(state, type, payload) {
  const payloadBuf = typeof payload === "string" ? Buffer.from(payload, "utf8") : payload;
  const frame = Buffer.allocUnsafe(HEADER_SIZE + payloadBuf.length);
  const seq = ++state.seqCounter;
  frame.writeUInt8(type, 0);
  frame.writeUInt32BE(seq, 1);
  frame.writeUInt32BE(state.highestAck, 5);
  frame.writeUInt32BE(payloadBuf.length, 9);
  if (payloadBuf.length > 0) {payloadBuf.copy(frame, HEADER_SIZE);}
  return frame;
}
function decodeFrame(state, buf) {
  if (buf.length < HEADER_SIZE) {return null;}
  const type = buf.readUInt8(0);
  const seq = buf.readUInt32BE(1);
  const ack = buf.readUInt32BE(5);
  const length = buf.readUInt32BE(9);
  const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + length);
  if (seq > state.highestAck) {state.highestAck = seq;}
  return { type, seq, ack, length, payload };
}
function parseJsonPayload(payload) {
  if (payload.length === 0) {return null;}
  try {
    return JSON.parse(payload.toString("utf8"));
  } catch {
    return null;
  }
}
var HEADER_SIZE;
var init_agent_wire = __esm({
  "src/relay/agent-wire.ts"() {
    "use strict";
    init_relay_protocol();
    HEADER_SIZE = 13;
  }
});

// src/shared/agent-wire-protocol.ts
var AGENT_HANDSHAKE_METHOD, AGENT_KEEPALIVE_INTERVAL_MS, AgentErrorCode;
var init_agent_wire_protocol = __esm({
  "src/shared/agent-wire-protocol.ts"() {
    "use strict";
    AGENT_HANDSHAKE_METHOD = "agent.handshake";
    AGENT_KEEPALIVE_INTERVAL_MS = 5e3;
    AgentErrorCode = {
      // JSON-RPC 2.0 standard
      ParseError: -32700,
      InvalidRequest: -32600,
      MethodNotFound: -32601,
      InvalidParams: -32602,
      ServerError: -32e3,
      // Agent-specific
      CommandNotFound: -33001,
      PermissionDenied: -33002,
      PathNotFound: -33003,
      PtyAllocationFailed: -33004,
      DiskFull: -33005,
      TooManyStreams: -33006,
      StreamProtocolError: -33007,
      HandshakeFailed: -33100,
      // Version mismatch, protocol violation
      AuthFailed: -33101
      // Invalid or missing agent token
    };
  }
});

// src/shared/trace/index.ts
function isTraceEnabled() {
  return _isEnabled();
}
function shortId() {
  return Math.random().toString(36).slice(2, 8);
}
function serializeFields(fields) {
  return Object.entries(fields).filter(([, v]) => v !== void 0).map(([k, v]) => {
    const s = String(v);
    return s.includes(" ") ? `${k}='${s}'` : `${k}=${s}`;
  }).join(" ");
}
function formatError(err) {
  if (err instanceof Error) {return err.message;}
  return String(err);
}
function emit(event) {
  const extra = serializeFields(event.fields);
  const extraStr = extra ? ` ${  extra}` : "";
  if (event.level === "start") {
    if (isTraceEnabled()) {
      console.log(`[TRACE] ${event.flow} id=${event.id}${extraStr}`);
    }
  } else if (event.level === "step") {
    if (isTraceEnabled()) {
      console.log(`[TRACE] ${event.flow} id=${event.id} step=${event.label ?? ""}${extraStr}`);
    }
  } else if (event.level === "ok") {
    if (isTraceEnabled()) {
      const elapsed = event.elapsedMs !== void 0 ? ` durationMs=${event.elapsedMs}` : "";
      console.log(`[TRACE] ${event.flow} id=${event.id} OK${extraStr}${elapsed}`);
    }
  } else {
    const elapsed = event.elapsedMs !== void 0 ? ` durationMs=${event.elapsedMs}` : "";
    console.error(`[TRACE] ${event.flow} id=${event.id} FAIL${extraStr}${elapsed}`);
  }
  for (const sink of sinks) {
    try {
      sink(event);
    } catch {
    }
  }
}
function createTracer(flow) {
  return {
    start(fields = {}, resume) {
      const id = resume?.id ?? shortId();
      const startMs = Date.now();
      emit({ id, flow, level: "start", fields, ts: startMs });
      return {
        id,
        step(label, stepFields = {}) {
          emit({
            id,
            flow,
            level: "step",
            label,
            fields: stepFields,
            ts: Date.now(),
            elapsedMs: Date.now() - startMs
          });
        },
        ok(okFields = {}) {
          emit({
            id,
            flow,
            level: "ok",
            fields: okFields,
            ts: Date.now(),
            elapsedMs: Date.now() - startMs
          });
        },
        fail(err, failFields = {}) {
          const errMsg = formatError(err);
          emit({
            id,
            flow,
            level: "fail",
            fields: { err: errMsg, ...failFields },
            ts: Date.now(),
            elapsedMs: Date.now() - startMs
          });
        }
      };
    }
  };
}
var sinks, _isEnabled;
var init_trace = __esm({
  "src/shared/trace/index.ts"() {
    "use strict";
    sinks = [];
    _isEnabled = () => {
      if (typeof process !== "undefined" && process.env) {
        const v = process.env["ORCA_TRACE"] ?? "";
        return v === "1" || v === "true" || v === "*";
      }
      return false;
    };
  }
});

// src/shared/trace/tracers.ts
var Tracers;
var init_tracers = __esm({
  "src/shared/trace/tracers.ts"() {
    "use strict";
    init_trace();
    Tracers = {
      /** Browser → RPC → IPC → Relay → Agent: directory browse */
      browseDirFlow: createTracer("devServer:browseDir"),
      /** Browser → RPC → IPC → Relay → Agent: mkdir */
      mkdirFlow: createTracer("devServer:mkdir"),
      /** Browser → RPC → IPC → Relay → Agent: rmdir */
      rmdirFlow: createTracer("devServer:rmdir"),
      /** Agent WebSocket lifecycle (connect / disconnect) */
      agentWsFlow: createTracer("agentWs:lifecycle"),
      /** IPC proxy call from user-process to main-process */
      ipcProxyFlow: createTracer("ipc:devServerProxy"),
      /** DiffViewer: load diff content (staged or unstaged) */
      codeReviewDiffFlow: createTracer("ui:codeReview.diff"),
      /** CodeReviewPanel: annotate a diff line */
      codeReviewAnnotateFlow: createTracer("ui:codeReview.annotate"),
      /** CodeReviewPanel: send review feedback */
      codeReviewFeedbackFlow: createTracer("ui:codeReview.sendFeedback"),
      /** CodeReviewPanel: generate AI commit message */
      codeReviewAiCommitFlow: createTracer("ui:codeReview.aiCommitMessage"),
      /** CodeReviewPanel: create pull request */
      codeReviewCreatePrFlow: createTracer("ui:codeReview.createPr"),
      // ─── CR-TRACE-003: Terminal Management (agent-side PTY only) ────────────────
      /** BL-TM-01/02 — create PTY (agent-PTY pty.create) */
      terminalCreate: createTracer("terminal:create"),
      /** BL-TM-02 — resize PTY */
      terminalResize: createTracer("terminal:resize"),
      /** BL-TM-03 — destroy/save-scrollback PTY */
      terminalDestroy: createTracer("terminal:destroy"),
      /** Reconnect to a still-running PTY after a WebSocket reconnect (pty.attach) */
      terminalReattach: createTracer("terminal:reattach"),
      // ─── CR-TRACE-001: Worktree Management (agent/backend-side, shared flow names) ─
      /** BL-WT-01 — create worktree (RPC handler + Agent git.worktree.add) */
      worktreeCreate: createTracer("worktree:create"),
      /** BL-WT-02 — fan out worktree to multiple agents — reserved, chưa có RPC method thật */
      worktreeFanOut: createTracer("worktree:fanOut"),
      /** BL-WT-03 — delete worktree (RPC handler + Agent git.worktree.remove) */
      worktreeDelete: createTracer("worktree:delete"),
      /** BL-WT-04 — compare worktree branches — reserved, chưa có RPC method thật */
      worktreeCompare: createTracer("worktree:compare"),
      /** BL-WT-05 — merge worktree branch — reserved, chưa có RPC method thật */
      worktreeMerge: createTracer("worktree:merge"),
      // ─── CR-TRACE-002: Agent Orchestration ──────────────────────────────────────
      /** BL-AG-01 — spawn AI agent (agent.exec / agent.spawn) */
      agentOrchSpawn: createTracer("agentOrch:spawn"),
      /** BL-AG-02 — stop agent (agent.kill / agent.sendInput Ctrl+C) */
      agentOrchStop: createTracer("agentOrch:stop"),
      /** BL-AG-03 — resume agent session (agent.spawn với resumeId) */
      agentOrchResume: createTracer("agentOrch:resume"),
      /** BL-AG-04 — switch account/provider (chưa có call site thật, đặt tên trước) */
      agentOrchSwitch: createTracer("agentOrch:switch"),
      /** BL-AG-05 — polling loop rời rạc (KHÔNG dùng cho agent.output stream) */
      agentOrchStatusPoll: createTracer("agentOrch:statusPoll"),
      // --- ui:* — tracer khởi tạo từ browser/renderer (CR-TRACE-015/016/017/018) ---
      /** Browser-initiated: mount ProfileEditor → fetch resolved + user profile (SOL-FE-TRACE-015 BL-PRF-02) */
      uiProfileResolveFlow: createTracer("ui:profile.resolve"),
      /** Browser-initiated: click "Save Changes" trong ProfileEditor (SOL-FE-TRACE-015 BL-PRF-01) */
      uiProfileUpdateFlow: createTracer("ui:profile.update"),
      /** Browser-initiated: click "Save" trong ProviderForm khi có credential mới (SOL-FE-TRACE-016 BL-AIP-01) */
      uiAiProviderWriteCredFlow: createTracer("ui:aiProvider.writeCredential"),
      /** Browser-initiated: click "Test" trên 1 provider account (SOL-FE-TRACE-016 BL-AIP-03) */
      uiAiProviderTestConnFlow: createTracer("ui:aiProvider.testConnection"),
      /** Browser-initiated: click "Save" trong WorkflowBuilder (SOL-FE-TRACE-017 BL-WF-01) */
      uiWorkflowTemplateSaveFlow: createTracer("ui:workflow.templateSave"),
      /** Browser-initiated: click "Run" — root span của execution nhìn từ browser (SOL-FE-TRACE-017 BL-WF-02) */
      uiWorkflowExecuteFlow: createTracer("ui:workflow.execute"),
      /** Browser-initiated: click "Cancel" trên execution đang chạy (SOL-FE-TRACE-017) */
      uiWorkflowCancelFlow: createTracer("ui:workflow.cancel"),
      /** Browser-initiated: click "Decompose with AI" trong TaskAIDecompose (SOL-FE-TRACE-018 BL-TG-02) */
      uiTaskGraphAiPlanFlow: createTracer("ui:taskGraph.aiPlan"),
      /** Browser-initiated: click "Execute/Run with Agent" — dùng chung bởi TaskDetail + TaskPromptEditor (SOL-FE-TRACE-018 BL-TG-04) */
      uiTaskGraphExecuteFlow: createTracer("ui:taskGraph.execute"),
      /** Browser-initiated: "New Worktree" dialog submit (SOL-FE-TRACE-001 BL-WT-01) — distinct
       *  from agent/backend-side `worktreeCreate` (`worktree:create`) so TracePanel's `isBackend`
       *  heuristic doesn't mislabel a browser-originated span as a backend event. */
      uiWorktreeCreateFlow: createTracer("ui:worktree.create"),
      /** Browser-initiated: "Delete Worktree" confirm (SOL-FE-TRACE-001 BL-WT-03) — distinct from
       *  agent/backend-side `worktreeDelete` (`worktree:delete`), same reasoning as above. */
      uiWorktreeDeleteFlow: createTracer("ui:worktree.delete"),
      /** BL-WT-02 — fan out worktree to multiple agents — reserved, chưa có call site */
      uiWorktreeFanOutFlow: createTracer("ui:worktree.fanOut"),
      /** BL-WT-04 — compare worktree branches — reserved, chưa có call site */
      uiWorktreeCompareFlow: createTracer("ui:worktree.compare"),
      /** BL-WT-05 — merge worktree branch — reserved, chưa có call site */
      uiWorktreeMergeFlow: createTracer("ui:worktree.merge"),
      // ─── CR-TRACE-002: Agent Orchestration (renderer-initiated, ui: prefix) ─────
      // Why: distinct from agentOrchSpawn/Stop/Resume/Switch/StatusPoll above
      // (agent-domain-side, shared non-prefixed flow names) — same isBackend
      // mislabeling concern as the worktree ui:* entries above.
      /** BL-AG-01 — spawn AI agent (AgentPanel.tsx start, orphan component — not mounted) */
      uiAgentOrchSpawnFlow: createTracer("ui:agentOrch.spawn"),
      /** BL-AG-02 — stop agent (AgentPanel.tsx stop, orphan component — not mounted) */
      uiAgentOrchStopFlow: createTracer("ui:agentOrch.stop"),
      /** BL-AG-03 — resume agent session (AgentPanel.tsx resume, orphan component — not mounted) */
      uiAgentOrchResumeFlow: createTracer("ui:agentOrch.resume"),
      /** BL-AG-04 — switch account/provider — chưa có UI, đặt tên sẵn */
      uiAgentOrchSwitchFlow: createTracer("ui:agentOrch.switch"),
      /** BL-AG-05 — polling loop rời rạc — dự phòng, không dùng làm span riêng (xem TASK-FE-002.3, dùng chung span mở) */
      uiAgentOrchStatusPollFlow: createTracer("ui:agentOrch.statusPoll"),
      // ─── CR-TRACE-014: Remote Integration (Backend-side only) ─────────────────
      /** BL-INT-01 (phần Main): đọc + giải mã PAT cho gh/glab trước khi Dev Server
       *  dùng để build env cho CLI. KHÔNG bao gồm bước gh/glab auth status thật —
       *  đó là remoteIntegration:ghExec, chạy trên Dev Server (companion solution). */
      remoteIntegrationCredentialDecryptFlow: createTracer("remoteIntegration:credentialDecrypt"),
      /** BL-INT-02: store/revoke token qua credentials.set/credentials.revoke RPC */
      remoteIntegrationCredentialStoreFlow: createTracer("remoteIntegration:credentialStore"),
      /** BL-INT-03: preflight check (local host hoặc relay-delegated) */
      remoteIntegrationPreflightFlow: createTracer("remoteIntegration:preflight"),
      // ─── CR-TRACE-014: Remote Integration (renderer-initiated, ui: prefix) ─────
      // Why: TASK-FE-014.1/014.2 originally proposed bare `remoteIntegrationPreflightFlow`/
      // `remoteIntegrationCredentialStoreFlow` keys, but a concurrent backend task already
      // claimed those exact key names above (`remoteIntegration:preflight`/
      // `remoteIntegration:credentialStore`). Per the no-rename collision rule, the
      // renderer-initiated `ui:*` variants use the `ui` prefix — same pattern as
      // `uiWorktreeCreateFlow`/`uiAgentOrchSpawnFlow`/`uiTerminalCreateFlow` above.
      /** BL-INT-01 + BL-INT-03: click "Re-check" → refreshPreflightStatus({ force: true }) —
       *  single shared renderer entry point (usePreflightCardStatuses + auto triggers). */
      uiRemoteIntegrationPreflightFlow: createTracer("ui:remoteIntegration.preflight"),
      /** BL-INT-02: CredentialInputForm.tsx handleSave/handleRevoke — orphan component,
       *  not mounted yet (TASK-FE-014.2). */
      uiRemoteIntegrationCredentialStoreFlow: createTracer("ui:remoteIntegration.credentialStore"),
      // ─── CR-TRACE-005: Code Review (Backend-side, `codeReview:` prefix per
      // CR-TRACE-000 §4) — NOTE naming drift: the `codeReview*Flow` keys above
      // (`codeReviewDiffFlow`/`codeReviewAnnotateFlow`/`codeReviewFeedbackFlow`/
      // `codeReviewAiCommitFlow`/`codeReviewCreatePrFlow`) were already claimed by
      // a concurrent frontend task for browser-initiated `ui:codeReview.*` flows.
      // Per the no-rename collision rule, backend entries below use bare
      // (no-`Flow`-suffix) keys — matching the sibling backend convention
      // (`worktreeCreate`, `agentOrchSpawn`, `terminalCreate`) — instead of the
      // key names originally proposed in TASK-BE-005.1/SOL-BE-TRACE-005. ─────────
      /** BL-CR-01: xem diff của agent changes (local + remote) */
      codeReviewDiff: createTracer("codeReview:diff"),
      /** BL-CR-02: annotate dòng code — KHÔNG wire vào code cho tới khi
       *  annotation.create RPC method + AgentManager.injectAnnotations() tồn tại
       *  (BUG-AG-ORCH-005). Khai báo trước theo CR-TRACE-000 §4 naming convention. */
      codeReviewAnnotate: createTracer("codeReview:annotate"),
      /** BL-CR-03: gửi feedback về agent — KHÔNG wire vào code cho tới khi
       *  review.sendFeedback RPC method tồn tại (BUG-AG-ORCH-001). */
      codeReviewFeedback: createTracer("codeReview:sendFeedback"),
      /** BL-CR-04: tạo commit message bằng AI (local + remote) */
      codeReviewAiCommit: createTracer("codeReview:aiCommitMessage"),
      /** BL-CR-05: tạo Pull Request với AI (local + remote) */
      codeReviewCreatePr: createTracer("codeReview:createPr"),
      // ─── CR-TRACE-013: Agent WebSocket (handshake/auth phase) ─────────────────
      /** BL-AWS-01: Orca initiator handshake (relay-websocket mode) — TCP connect
       *  + agent.handshake round-trip, TRƯỚC khi agentWs:lifecycle bắt đầu. */
      agentWsHandshakeFlow: createTracer("agentWs:handshake"),
      /** BL-AWS-02: Orca receiver handshake + token validation (direct-websocket
       *  mode) — từ lúc socket upgrade tới accept/reject, TRƯỚC agentWs:lifecycle. */
      agentWsTokenVerifyFlow: createTracer("agentWs:tokenVerify"),
      // ─── CR-TRACE-003: Terminal Management (renderer-initiated, ui: prefix) ────
      // Why: distinct from terminalCreate/terminalResize/terminalDestroy/
      // terminalReattach above (agent-side PTY-only, shared non-prefixed flow
      // names) — same isBackend mislabeling concern as the other ui:* entries.
      /** BL-TM-01 — create PTY session (createRemoteRuntimePtyTransport connect()) */
      uiTerminalCreateFlow: createTracer("ui:terminal.create"),
      /** BL-TM-02 — resize/claim viewport */
      uiTerminalResizeFlow: createTracer("ui:terminal.resize"),
      /** BL-TM-03 — destroy PTY / save scrollback */
      uiTerminalDestroyFlow: createTracer("ui:terminal.destroy"),
      /** BL-TM-03 restore — reconnect a still-running PTY — chưa có call site rõ ràng, đặt tên sẵn */
      uiTerminalReconnectFlow: createTracer("ui:terminal.reconnect"),
      // ─── CR-TRACE-015: Profile & Project (Backend-side) ────────────────────────
      /** BL-PRF-01: update company/dept/user profile + cache invalidate */
      profileUpdateLayerFlow: createTracer("profile:updateLayer"),
      /** BL-PRF-02: 3-layer resolve (cache hit/miss + merge, không trace merge() nội bộ) */
      profileResolveFlow: createTracer("profile:resolve"),
      /** BL-PRF-03: project create + dev-server relay routing (field `op` phân biệt) */
      profileProjectRouteFlow: createTracer("profile:projectRoute"),
      /** BL-PRF-04: profile-aware agent spawn orchestration (assertAccess prep TRƯỚC
       *  ProfileAwareAgentSpawner.spawn(), resume vào agentOrch:spawn — CR-TRACE-002) */
      profileAgentSpawnFlow: createTracer("profile:agentSpawnRoute"),
      // ─── CR-TRACE-016: AI Provider Management (Backend-side) ───────────────────
      /** BL-AIP-01: write encrypted credential to dev server via relay */
      aiProviderWriteCredFlow: createTracer("aiProvider:writeCredential"),
      /** BL-AIP-02: priority + quota resolution cho agent/workflow spawn */
      aiProviderResolveFlow: createTracer("aiProvider:resolve"),
      /** BL-AIP-03: background health check cron (15 phút/lần) */
      aiProviderHealthFlow: createTracer("aiProvider:healthCheck"),
      // ─── CR-TRACE-018: Task Graph (Backend-side) ────────────────────────────────
      /** BL-TG-01: add dependency edge — cycle detection (DFS thật, không phải BFS) là phần đáng trace nhất */
      taskGraphEdgeFlow: createTracer("taskGraph:addEdge"),
      /** BL-TG-02: AI decompose — tách rõ "AI call chậm" vs "parse JSON lỗi" */
      taskGraphAiPlanFlow: createTracer("taskGraph:aiPlan"),
      /** BL-TG-03: multi-level ancestor grant resolution — chạy trên mọi permission check */
      taskGraphGrantFlow: createTracer("taskGraph:grantResolve"),
      /** BL-TG-04: task prompt → agent execution, resume vào agentOrch:spawn (CR-TRACE-002) */
      taskGraphExecuteFlow: createTracer("taskGraph:execute"),
      // ─── CR-TRACE-017: Workflow Orchestration (Backend-side) ───────────────────
      /** BL-WF-01: template create/inherit */
      workflowTemplateCreateFlow: createTracer("workflow:templateCreate"),
      /** BL-WF-02: span CHA — 1 per execution, sống suốt vòng đời execution */
      workflowExecuteFlow: createTracer("workflow:execute"),
      /** BL-WF-02: span CON — 1 per step, mang field parentTraceId để group theo execution */
      workflowStepFlow: createTracer("workflow:stepExecute"),
      /** BL-WF-03: PLACEHOLDER — chưa có implementation, TemplateResolver.ts không có
       *  updateVisibility()/share-token/shared route nào trong code hiện tại. Khai báo tên
       *  tracer để sẵn sàng khi tính năng sharing tồn tại, KHÔNG viết call site nào cho nó. */
      workflowShareFlow: createTracer("workflow:share")
    };
  }
});

// src/relay/context.ts
function expandTilde(p) {
  if (p === "~" || p === "~/" || p === "~\\") {
    return (0, import_node_os2.homedir)();
  }
  if (p.startsWith("~/")) {
    return (0, import_node_path3.resolve)((0, import_node_os2.homedir)(), p.slice(2));
  }
  if (p.startsWith("~\\")) {
    return `${(0, import_node_os2.homedir)()}\\${p.slice(2)}`;
  }
  return p;
}
var import_node_path3, import_node_os2;
var init_context = __esm({
  "src/relay/context.ts"() {
    "use strict";
    import_node_path3 = require("node:path");
    import_node_os2 = require("node:os");
  }
});

// src/shared/git-cquoted-path.ts
function decodeGitCQuotedPath(value) {
  if (value.length < 2 || value[0] !== '"' || value.at(-1) !== '"') {
    return value;
  }
  let decoded = "";
  for (let index = 1; index < value.length - 1; index += 1) {
    const char = value[index];
    if (char !== "\\") {
      decoded += char;
      continue;
    }
    index += 1;
    const escaped = value[index];
    switch (escaped) {
      case "a":
        decoded += "\x07";
        break;
      case "b":
        decoded += "\b";
        break;
      case "f":
        decoded += "\f";
        break;
      case "n":
        decoded += "\n";
        break;
      case "r":
        decoded += "\r";
        break;
      case "t":
        decoded += "	";
        break;
      case "v":
        decoded += "\v";
        break;
      case "\\":
      case '"':
        decoded += escaped;
        break;
      default:
        if (/[0-7]/.test(escaped)) {
          const bytes = [];
          let octalStart = index;
          while (octalStart < value.length - 1) {
            let octal = value[octalStart];
            let octalEnd = octalStart;
            while (octalEnd + 1 < value.length - 1 && octal.length < 3 && /[0-7]/.test(value[octalEnd + 1])) {
              octalEnd += 1;
              octal += value[octalEnd];
            }
            bytes.push(Number.parseInt(octal, 8));
            index = octalEnd;
            if (value[index + 1] !== "\\" || !/[0-7]/.test(value[index + 2] ?? "")) {
              break;
            }
            octalStart = index + 2;
          }
          decoded += new TextDecoder("utf-8", { ignoreBOM: true }).decode(Uint8Array.from(bytes));
        } else {
          decoded += escaped;
        }
        break;
    }
  }
  return decoded;
}
var init_git_cquoted_path = __esm({
  "src/shared/git-cquoted-path.ts"() {
    "use strict";
  }
});

// src/shared/binary-buffer.ts
function isBinaryBuffer(buffer) {
  const len = Math.min(buffer.length, BINARY_SNIFF_BYTES);
  for (let i = 0; i < len; i += 1) {
    if (buffer[i] === 0) {
      return true;
    }
  }
  return false;
}
var BINARY_SNIFF_BYTES;
var init_binary_buffer = __esm({
  "src/shared/binary-buffer.ts"() {
    "use strict";
    BINARY_SNIFF_BYTES = 8192;
  }
});

// src/shared/git-worktree-command-capabilities.ts
function getGitErrorText(error) {
  if (typeof error !== "object" || error === null) {
    return error instanceof Error ? error.message : String(error);
  }
  const values = ["message", "stderr", "stdout"].map((key) => error[key]).filter((value) => typeof value === "string");
  return values.join("\n");
}
function getGitErrorCode(error) {
  return typeof error === "object" && error !== null && "code" in error ? String(error.code) : void 0;
}
function isUnsupportedWorktreeListZError(error) {
  if (getGitErrorCode(error) === "129") {
    return true;
  }
  return /(?:unknown|invalid|unrecognized) (?:switch|option).*`?-?z'?/i.test(getGitErrorText(error));
}
function isUnsupportedRevParsePathFormatError(error) {
  return /(?:unknown|invalid|unrecognized).*(?:--path-format|path-format)/i.test(
    getGitErrorText(error)
  );
}
function hasUnsupportedRevParsePathFormatEcho(output) {
  return output.split(/\r?\n/).some((line) => line.startsWith("--path-format"));
}
var init_git_worktree_command_capabilities = __esm({
  "src/shared/git-worktree-command-capabilities.ts"() {
    "use strict";
  }
});

// src/relay/git-handler-utils.ts
function parseBranchStatusChar(char) {
  switch (char) {
    case "M":
      return "modified";
    case "A":
      return "added";
    case "D":
      return "deleted";
    case "R":
      return "renamed";
    case "C":
      return "copied";
    default:
      return "modified";
  }
}
function parseConflictKind(xy) {
  switch (xy) {
    case "UU":
      return "both_modified";
    case "AA":
      return "both_added";
    case "DD":
      return "both_deleted";
    case "AU":
      return "added_by_us";
    case "UA":
      return "added_by_them";
    case "DU":
      return "deleted_by_us";
    case "UD":
      return "deleted_by_them";
    default:
      return null;
  }
}
function parseUnmergedEntry(worktreePath, line) {
  const parts = line.split(" ");
  const xy = parts[1];
  const modeStage1 = parts[3];
  const modeStage2 = parts[4];
  const modeStage3 = parts[5];
  const filePath = parts.slice(10).join(" ");
  if (!filePath) {
    return null;
  }
  if ([modeStage1, modeStage2, modeStage3].some((m) => m === "160000")) {
    return null;
  }
  const conflictKind = parseConflictKind(xy);
  if (!conflictKind) {
    return null;
  }
  let status = "modified";
  if (conflictKind === "both_deleted") {
    status = "deleted";
  } else if (conflictKind !== "both_modified" && conflictKind !== "both_added") {
    try {
      status = (0, import_node_fs2.existsSync)(path.join(worktreePath, filePath)) ? "modified" : "deleted";
    } catch {
      status = "modified";
    }
  }
  return {
    path: filePath,
    area: "unstaged",
    status,
    conflictKind,
    conflictStatus: "unresolved"
  };
}
function parseBranchDiff(stdout, statsByPath = /* @__PURE__ */ new Map()) {
  const entries = [];
  for (const line of stdout.split(/\r?\n/)) {
    if (!line) {
      continue;
    }
    const parts = line.split("	");
    const rawStatus = parts[0] ?? "";
    const status = parseBranchStatusChar(rawStatus[0] ?? "M");
    if (rawStatus.startsWith("R") || rawStatus.startsWith("C")) {
      const oldPath = parts[1];
      const filePath = parts[2];
      if (filePath) {
        entries.push({ path: filePath, oldPath, status, ...statsByPath.get(filePath) });
      }
    } else {
      const filePath = parts[1];
      if (filePath) {
        entries.push({ path: filePath, status, ...statsByPath.get(filePath) });
      }
    }
  }
  return entries;
}
function parseWorktreeList(output, options = {}) {
  const worktrees = [];
  const blocks = options.nulDelimited ? splitNulWorktreeList(output) : splitLineWorktreeList(output);
  for (const lines of blocks) {
    if (lines.length === 0) {
      continue;
    }
    let wtPath = "";
    let head = "";
    let branch = "";
    let isBare = false;
    let locked = false;
    let lockReason = "";
    for (const line of lines) {
      if (line.startsWith("worktree ")) {
        wtPath = line.slice("worktree ".length);
      } else if (line.startsWith("HEAD ")) {
        head = line.slice("HEAD ".length);
      } else if (line.startsWith("branch ")) {
        branch = line.slice("branch ".length);
      } else if (line === "bare") {
        isBare = true;
      } else if (line === "locked" || line.startsWith("locked ")) {
        locked = true;
        const rawReason = line.slice("locked".length).trim();
        lockReason = options.nulDelimited ? rawReason : decodeGitCQuotedPath(rawReason);
      }
    }
    if (wtPath) {
      worktrees.push({
        path: wtPath,
        head,
        branch,
        isBare,
        ...locked ? { locked: true } : {},
        ...lockReason ? { lockReason } : {},
        isMainWorktree: worktrees.length === 0
      });
    }
  }
  return worktrees;
}
function splitLineWorktreeList(output) {
  return output.trim().split(/\r?\n\r?\n/).map((block) => block.trim().split(/\r?\n/));
}
function splitNulWorktreeList(output) {
  if (!output.includes("\0")) {
    return splitLineWorktreeList(output);
  }
  const blocks = [];
  let currentBlock = [];
  for (const field of output.split("\0")) {
    if (field) {
      currentBlock.push(field);
      continue;
    }
    if (currentBlock.length > 0) {
      blocks.push(currentBlock);
      currentBlock = [];
    }
  }
  if (currentBlock.length > 0) {
    blocks.push(currentBlock);
  }
  return blocks;
}
function bufferToBlob(buffer, filePath) {
  const binary = isBinaryBuffer(buffer);
  if (binary) {
    const ext = filePath ? path.extname(filePath).toLowerCase() : "";
    const previewable = !!PREVIEWABLE_MIME[ext];
    return { content: previewable ? buffer.toString("base64") : "", isBinary: true };
  }
  return { content: buffer.toString("utf-8"), isBinary: false };
}
var import_node_fs2, path, PREVIEWABLE_MIME;
var init_git_handler_utils = __esm({
  "src/relay/git-handler-utils.ts"() {
    "use strict";
    import_node_fs2 = require("node:fs");
    path = __toESM(require("node:path"));
    init_git_cquoted_path();
    init_binary_buffer();
    init_git_worktree_command_capabilities();
    PREVIEWABLE_MIME = {
      ".png": "image/png",
      ".jpg": "image/jpeg",
      ".jpeg": "image/jpeg",
      ".gif": "image/gif",
      ".svg": "image/svg+xml",
      ".webp": "image/webp",
      ".bmp": "image/bmp",
      ".ico": "image/x-icon",
      ".pdf": "application/pdf"
    };
  }
});

// src/shared/git-status-limit.ts
var DEFAULT_GIT_STATUS_LIMIT;
var init_git_status_limit = __esm({
  "src/shared/git-status-limit.ts"() {
    "use strict";
    DEFAULT_GIT_STATUS_LIMIT = 1e4;
  }
});

// src/shared/git-uncommitted-line-stats.ts
function parseNumstatCount(value) {
  if (value === "-") {
    return void 0;
  }
  const count = Number.parseInt(value, 10);
  return Number.isFinite(count) ? count : void 0;
}
function normalizeNumstatPath(rawPath) {
  const decodedPath = decodeGitCQuotedPath(rawPath);
  const braced = /^(.*)\{(.+) => (.+)\}(.*)$/.exec(decodedPath);
  if (braced) {
    return `${braced[1]}${braced[3]}${braced[4]}`;
  }
  const marker = " => ";
  const markerIndex = decodedPath.lastIndexOf(marker);
  return markerIndex === -1 ? decodedPath : decodedPath.slice(markerIndex + marker.length);
}
function parseNumstat(stdout) {
  if (stdout.includes("\0")) {
    return parseNulDelimitedNumstat(stdout);
  }
  const stats = /* @__PURE__ */ new Map();
  for (const line of stdout.split(/\r?\n/)) {
    if (!line) {
      continue;
    }
    const parts = line.split("	");
    const rawPath = parts.slice(2).join("	");
    if (!rawPath) {
      continue;
    }
    stats.set(normalizeNumstatPath(rawPath), {
      added: parseNumstatCount(parts[0] ?? ""),
      removed: parseNumstatCount(parts[1] ?? "")
    });
  }
  return stats;
}
function parseNulDelimitedNumstat(stdout) {
  const stats = /* @__PURE__ */ new Map();
  const records = stdout.split("\0");
  for (let i = 0; i < records.length; i += 1) {
    const record = records[i];
    if (!record) {
      continue;
    }
    const parts = record.split("	");
    const rawPath = parts.slice(2).join("	");
    let path12 = rawPath;
    if (!path12) {
      i += 2;
      path12 = records[i] ?? "";
    }
    if (!path12) {
      continue;
    }
    stats.set(path12, {
      added: parseNumstatCount(parts[0] ?? ""),
      removed: parseNumstatCount(parts[1] ?? "")
    });
  }
  return stats;
}
async function countFileAdditions(absolutePath) {
  try {
    const fileStat = await (0, import_promises2.lstat)(absolutePath);
    const cached = untrackedStatsCache.get(absolutePath);
    if (cached && cached.size === fileStat.size && cached.mtimeMs === fileStat.mtimeMs && cached.ctimeMs === fileStat.ctimeMs) {
      untrackedStatsCache.delete(absolutePath);
      untrackedStatsCache.set(absolutePath, cached);
      return cached.stats;
    }
    if (fileStat.isSymbolicLink()) {
      return rememberUntrackedStats(absolutePath, fileStat, { added: 1 });
    }
    if (!fileStat.isFile() || fileStat.size > MAX_UNTRACKED_LINE_COUNT_BYTES) {
      return rememberUntrackedStats(absolutePath, fileStat, {});
    }
    const buffer = await (0, import_promises2.readFile)(absolutePath);
    if (isBinaryBuffer(buffer)) {
      return rememberUntrackedStats(absolutePath, fileStat, {});
    }
    if (buffer.length === 0) {
      return rememberUntrackedStats(absolutePath, fileStat, { added: 0 });
    }
    let newlineCount = 0;
    for (let i = 0; i < buffer.length; i += 1) {
      if (buffer[i] === NEWLINE_BYTE) {
        newlineCount += 1;
      }
    }
    const endsWithNewline = buffer.at(-1) === NEWLINE_BYTE;
    return rememberUntrackedStats(absolutePath, fileStat, {
      added: endsWithNewline ? newlineCount : newlineCount + 1
    });
  } catch {
    return {};
  }
}
function rememberUntrackedStats(absolutePath, fileStat, stats) {
  untrackedStatsCache.delete(absolutePath);
  untrackedStatsCache.set(absolutePath, {
    size: fileStat.size,
    mtimeMs: fileStat.mtimeMs,
    ctimeMs: fileStat.ctimeMs,
    stats
  });
  if (untrackedStatsCache.size > UNTRACKED_STATS_CACHE_MAX_ENTRIES) {
    const oldestKey = untrackedStatsCache.keys().next().value;
    if (oldestKey) {
      untrackedStatsCache.delete(oldestKey);
    }
  }
  return stats;
}
async function collectUntrackedAdditions(worktreePath, untrackedPaths) {
  const result = /* @__PURE__ */ new Map();
  for (let i = 0; i < untrackedPaths.length; i += UNTRACKED_READ_CONCURRENCY) {
    const chunk = untrackedPaths.slice(i, i + UNTRACKED_READ_CONCURRENCY);
    await Promise.all(
      chunk.map(async (relativePath) => {
        result.set(relativePath, await countFileAdditions(path2.join(worktreePath, relativePath)));
      })
    );
  }
  return result;
}
function applyLineStats(entry, stats) {
  if (!stats) {
    return;
  }
  if (stats.added !== void 0) {
    entry.added = stats.added;
  }
  if (stats.removed !== void 0) {
    entry.removed = stats.removed;
  }
}
var import_promises2, path2, UNTRACKED_READ_CONCURRENCY, MAX_UNTRACKED_LINE_COUNT_BYTES, UNTRACKED_STATS_CACHE_MAX_ENTRIES, NEWLINE_BYTE, untrackedStatsCache;
var init_git_uncommitted_line_stats = __esm({
  "src/shared/git-uncommitted-line-stats.ts"() {
    "use strict";
    import_promises2 = require("node:fs/promises");
    path2 = __toESM(require("node:path"));
    init_binary_buffer();
    init_git_cquoted_path();
    init_git_status_limit();
    UNTRACKED_READ_CONCURRENCY = 8;
    MAX_UNTRACKED_LINE_COUNT_BYTES = 2 * 1024 * 1024;
    UNTRACKED_STATS_CACHE_MAX_ENTRIES = 2 * DEFAULT_GIT_STATUS_LIMIT;
    NEWLINE_BYTE = 10;
    untrackedStatsCache = /* @__PURE__ */ new Map();
  }
});

// src/shared/large-diff-render-limit.ts
function countLinesEmptyAsZeroUpToLimit(content, maxLines) {
  if (content.length === 0) {
    return { count: 0, exceeded: false };
  }
  let lineCount = 1;
  for (let index = 0; index < content.length; index += 1) {
    if (content.charCodeAt(index) !== 10) {
      continue;
    }
    lineCount += 1;
    if (lineCount > maxLines) {
      return { count: lineCount, exceeded: true };
    }
  }
  return { count: lineCount, exceeded: false };
}
function getLargeDiffRenderLimit({
  originalContent,
  modifiedContent
}) {
  const characterCount = originalContent.length + modifiedContent.length;
  const limits = {
    maxLinesPerSide: MAX_RENDERED_DIFF_LINES_PER_SIDE,
    maxCombinedCharacters: MAX_RENDERED_DIFF_COMBINED_CHARACTERS
  };
  if (characterCount > MAX_RENDERED_DIFF_COMBINED_CHARACTERS) {
    return {
      limited: true,
      reason: "character-count",
      lineCounts: null,
      characterCount,
      limits
    };
  }
  const originalLineCount = countLinesEmptyAsZeroUpToLimit(
    originalContent,
    MAX_RENDERED_DIFF_LINES_PER_SIDE
  );
  const modifiedLineCount = countLinesEmptyAsZeroUpToLimit(
    modifiedContent,
    MAX_RENDERED_DIFF_LINES_PER_SIDE
  );
  if (originalLineCount.exceeded || modifiedLineCount.exceeded) {
    return {
      limited: true,
      reason: "line-count",
      lineCounts: {
        original: originalLineCount.count,
        modified: modifiedLineCount.count
      },
      lineCountsAreMinimum: {
        original: originalLineCount.exceeded,
        modified: modifiedLineCount.exceeded
      },
      characterCount,
      limits
    };
  }
  return {
    limited: false,
    lineCounts: {
      original: originalLineCount.count,
      modified: modifiedLineCount.count
    },
    characterCount
  };
}
var MAX_RENDERED_DIFF_LINES_PER_SIDE, MAX_RENDERED_DIFF_COMBINED_CHARACTERS;
var init_large_diff_render_limit = __esm({
  "src/shared/large-diff-render-limit.ts"() {
    "use strict";
    MAX_RENDERED_DIFF_LINES_PER_SIDE = 12e4;
    MAX_RENDERED_DIFF_COMBINED_CHARACTERS = 6e6;
  }
});

// src/relay/git-diff-result.ts
function buildDiffResult(originalContent, modifiedContent, originalIsBinary, modifiedIsBinary, filePath) {
  if (originalIsBinary || modifiedIsBinary) {
    const ext = filePath ? path3.extname(filePath).toLowerCase() : "";
    const mimeType = PREVIEWABLE_MIME[ext];
    return {
      kind: "binary",
      originalContent,
      modifiedContent,
      originalIsBinary,
      modifiedIsBinary,
      ...mimeType ? { isImage: true, mimeType } : {}
    };
  }
  const largeDiffRenderLimit = getLargeDiffRenderLimit({ originalContent, modifiedContent });
  if (largeDiffRenderLimit.limited) {
    return {
      kind: "text",
      originalContent: "",
      modifiedContent: "",
      originalIsBinary: false,
      modifiedIsBinary: false,
      largeDiffRenderLimit
    };
  }
  return {
    kind: "text",
    originalContent,
    modifiedContent,
    originalIsBinary: false,
    modifiedIsBinary: false
  };
}
var path3;
var init_git_diff_result = __esm({
  "src/relay/git-diff-result.ts"() {
    "use strict";
    path3 = __toESM(require("node:path"));
    init_large_diff_render_limit();
    init_git_handler_utils();
  }
});

// src/relay/git-buffer-overflow.ts
function isGitBufferOverflowError(error) {
  if (!error || typeof error !== "object") {
    return false;
  }
  const maybeError = error;
  if (maybeError.code === "ENOBUFS") {
    return true;
  }
  return typeof maybeError.message === "string" && /\bmaxBuffer\b/i.test(maybeError.message);
}
var init_git_buffer_overflow = __esm({
  "src/relay/git-buffer-overflow.ts"() {
    "use strict";
  }
});

// src/relay/git-working-file-read.ts
async function readWorkingDiffFile(absPath) {
  try {
    const fileStat = await (0, import_promises3.stat)(absPath);
    if (!fileStat.isFile()) {
      return { content: "", isBinary: false };
    }
    if (fileStat.size > MAX_RELAY_DIFF_WORKING_FILE_BYTES) {
      return { content: "", isBinary: true };
    }
    const buffer = await (0, import_promises3.readFile)(absPath);
    return bufferToBlob(buffer);
  } catch {
    return { content: "", isBinary: false };
  }
}
var import_promises3, MAX_RELAY_DIFF_WORKING_FILE_BYTES;
var init_git_working_file_read = __esm({
  "src/relay/git-working-file-read.ts"() {
    "use strict";
    import_promises3 = require("node:fs/promises");
    init_git_handler_utils();
    MAX_RELAY_DIFF_WORKING_FILE_BYTES = 10 * 1024 * 1024;
  }
});

// src/relay/git-exec-validator.ts
function validateCloneArgs(args) {
  const allowed = args[1] === "--progress" ? args.slice(2) : args.slice(1);
  if (allowed.length !== 3 || allowed[0] !== "--") {
    throw new Error("git clone via exec is restricted to clone [--progress] -- <url> <dir>");
  }
  const targetDir = allowed[2];
  if (!targetDir || targetDir === "." || targetDir === ".." || targetDir.includes("/") || targetDir.includes("\\") || targetDir.includes("\0")) {
    throw new Error("git clone target directory must be a single safe path segment");
  }
}
function validateInitArgs(args) {
  if (args.length !== 1) {
    throw new Error("git init via exec is restricted to init with no arguments");
  }
}
function validateCommitArgs(args) {
  if (args.length !== 4 || args[1] !== "--allow-empty" || args[2] !== "-m" || !args[3]) {
    throw new Error("git commit via exec is restricted to commit --allow-empty -m <message>");
  }
}
function matchesDeniedFlag(arg, denySet) {
  if (denySet.has(arg)) {
    return true;
  }
  const eqIdx = arg.indexOf("=");
  if (eqIdx > 0) {
    return denySet.has(arg.slice(0, eqIdx));
  }
  return false;
}
function validateGitExecArgs(args) {
  let subcommandIdx = 0;
  while (subcommandIdx < args.length && args[subcommandIdx].startsWith("-")) {
    subcommandIdx++;
  }
  if (subcommandIdx > 0) {
    throw new Error("Global git flags before the subcommand are not allowed");
  }
  const subcommand = args[0];
  if (!subcommand || !ALLOWED_GIT_SUBCOMMANDS.has(subcommand)) {
    throw new Error(`git subcommand not allowed: ${subcommand ?? "(empty)"}`);
  }
  const restArgs = args.slice(1);
  if (restArgs.some((a) => matchesDeniedFlag(a, GLOBAL_DENIED_FLAGS))) {
    throw new Error("Dangerous git flags are not allowed via exec");
  }
  if (subcommand === "config") {
    if (!restArgs.some((a) => CONFIG_READ_ONLY_FLAGS.has(a))) {
      throw new Error("git config is restricted to read-only operations (--get, --list, etc.)");
    }
    if (restArgs.some((a) => matchesDeniedFlag(a, CONFIG_WRITE_FLAGS))) {
      throw new Error("git config write operations are not allowed via exec");
    }
  }
  if (subcommand === "init") {
    validateInitArgs(args);
  }
  if (subcommand === "commit") {
    validateCommitArgs(args);
  }
  if (subcommand === "branch") {
    if (restArgs.some((a) => matchesDeniedFlag(a, BRANCH_DESTRUCTIVE_FLAGS))) {
      throw new Error("Destructive git branch flags are not allowed via exec");
    }
  }
  if (subcommand === "remote") {
    const remoteSubcmd = restArgs.find((a) => !a.startsWith("-"));
    if (remoteSubcmd && REMOTE_WRITE_SUBCOMMANDS.has(remoteSubcmd)) {
      throw new Error("Destructive git remote operations are not allowed via exec");
    }
  }
  if (subcommand === "symbolic-ref") {
    if (restArgs.some((a) => matchesDeniedFlag(a, SYMBOLIC_REF_WRITE_FLAGS))) {
      throw new Error("git symbolic-ref write operations are not allowed via exec");
    }
    const positionalArgs = restArgs.filter((a) => !a.startsWith("-"));
    if (positionalArgs.length >= 2) {
      throw new Error("git symbolic-ref write operations are not allowed via exec");
    }
  }
  if (subcommand === "diff") {
    if (!restArgs.some((a) => a === "--cached" || a === "--staged")) {
      throw new Error("git diff via exec is restricted to staged changes");
    }
    const unsupportedArg = restArgs.find((a) => !DIFF_ALLOWED_FLAGS.has(a));
    if (unsupportedArg) {
      throw new Error(`git diff flag not allowed via exec: ${unsupportedArg}`);
    }
  }
  if (subcommand === "clone") {
    validateCloneArgs(args);
  }
}
var ALLOWED_GIT_SUBCOMMANDS, CONFIG_READ_ONLY_FLAGS, CONFIG_WRITE_FLAGS, BRANCH_DESTRUCTIVE_FLAGS, GLOBAL_DENIED_FLAGS, REMOTE_WRITE_SUBCOMMANDS, SYMBOLIC_REF_WRITE_FLAGS, DIFF_ALLOWED_FLAGS;
var init_git_exec_validator = __esm({
  "src/relay/git-exec-validator.ts"() {
    "use strict";
    ALLOWED_GIT_SUBCOMMANDS = /* @__PURE__ */ new Set([
      "rev-parse",
      "branch",
      "log",
      "show-ref",
      "ls-remote",
      "remote",
      "symbolic-ref",
      "merge-base",
      "diff",
      "ls-files",
      "clone",
      "init",
      "commit",
      "for-each-ref",
      "check-ref-format",
      "config"
    ]);
    CONFIG_READ_ONLY_FLAGS = /* @__PURE__ */ new Set(["--get", "--get-all", "--list", "--get-regexp", "-l"]);
    CONFIG_WRITE_FLAGS = /* @__PURE__ */ new Set([
      "--add",
      "--unset",
      "--unset-all",
      "--replace-all",
      "--rename-section",
      "--remove-section",
      "--edit",
      "-e",
      // Why: --file redirects config reads to an arbitrary file, enabling path
      // traversal (e.g. `--file /etc/passwd --list` leaks file contents).
      "--file",
      "-f",
      "--global",
      "--system"
    ]);
    BRANCH_DESTRUCTIVE_FLAGS = /* @__PURE__ */ new Set([
      "-d",
      "-D",
      "--delete",
      "-m",
      "-M",
      "--move",
      "-c",
      "-C",
      "--copy"
    ]);
    GLOBAL_DENIED_FLAGS = /* @__PURE__ */ new Set(["--output", "-o", "--exec-path", "--work-tree", "--git-dir"]);
    REMOTE_WRITE_SUBCOMMANDS = /* @__PURE__ */ new Set([
      "add",
      "remove",
      "rm",
      "rename",
      "set-head",
      "set-branches",
      "set-url",
      "prune",
      "update"
    ]);
    SYMBOLIC_REF_WRITE_FLAGS = /* @__PURE__ */ new Set(["-d", "--delete", "-m"]);
    DIFF_ALLOWED_FLAGS = /* @__PURE__ */ new Set([
      "--cached",
      "--staged",
      "--name-status",
      "--patch",
      "--minimal",
      "--no-color",
      "--no-ext-diff"
    ]);
  }
});

// src/relay/git-handler-ops.ts
async function readBlobAtOid(gitBuffer, cwd, oid, filePath) {
  const gitPath = filePath.replace(/\\/g, "/");
  try {
    const buf = await gitBuffer(["show", "--end-of-options", `${oid}:${gitPath}`], cwd);
    return bufferToBlob(buf, filePath);
  } catch (error) {
    if (isGitBufferOverflowError(error)) {
      return { content: "", isBinary: true };
    }
    return { content: "", isBinary: false };
  }
}
async function readBlobAtIndex(gitBuffer, cwd, filePath) {
  const gitPath = filePath.replace(/\\/g, "/");
  try {
    const buf = await gitBuffer(["show", "--end-of-options", `:${gitPath}`], cwd);
    return bufferToBlob(buf, filePath);
  } catch (error) {
    if (isGitBufferOverflowError(error)) {
      return { content: "", isBinary: true };
    }
    return { content: "", isBinary: false };
  }
}
async function readUnstagedLeft(gitBuffer, cwd, filePath) {
  const index = await readBlobAtIndex(gitBuffer, cwd, filePath);
  if (index.content || index.isBinary) {
    return index;
  }
  return readBlobAtOid(gitBuffer, cwd, "HEAD", filePath);
}
async function computeDiff(git, worktreePath, filePath, staged, compareAgainstHead = false) {
  let originalContent = "";
  let modifiedContent = "";
  let originalIsBinary = false;
  let modifiedIsBinary = false;
  try {
    if (staged) {
      const left = await readBlobAtOid(git, worktreePath, "HEAD", filePath);
      originalContent = left.content;
      originalIsBinary = left.isBinary;
      const right = await readBlobAtIndex(git, worktreePath, filePath);
      modifiedContent = right.content;
      modifiedIsBinary = right.isBinary;
    } else {
      const left = compareAgainstHead ? await readBlobAtOid(git, worktreePath, "HEAD", filePath) : await readUnstagedLeft(git, worktreePath, filePath);
      originalContent = left.content;
      originalIsBinary = left.isBinary;
      const right = await readWorkingDiffFile(path4.join(worktreePath, filePath));
      modifiedContent = right.content;
      modifiedIsBinary = right.isBinary;
    }
  } catch {
  }
  return buildDiffResult(
    originalContent,
    modifiedContent,
    originalIsBinary,
    modifiedIsBinary,
    filePath
  );
}
async function branchCompare(git, worktreePath, baseRef, loadBranchChanges) {
  const summary = {
    baseRef,
    baseOid: null,
    compareRef: "HEAD",
    headOid: null,
    mergeBase: null,
    changedFiles: 0,
    status: "loading"
  };
  try {
    const { stdout: branchOut } = await git(["branch", "--show-current"], worktreePath);
    const branch = branchOut.trim();
    if (branch) {
      summary.compareRef = branch;
    }
  } catch {
  }
  let headOid;
  let baseOid = "";
  try {
    const { stdout } = await git(["rev-parse", "--verify", "HEAD"], worktreePath);
    headOid = stdout.trim();
    summary.headOid = headOid;
  } catch {
    try {
      const { stdout } = await git(["rev-parse", "--verify", baseRef], worktreePath);
      baseOid = stdout.trim();
      summary.baseOid = baseOid;
      summary.changedFiles = 0;
      summary.commitsAhead = 0;
      summary.status = "ready";
      return { summary, entries: [] };
    } catch {
    }
    summary.status = "unborn-head";
    summary.errorMessage = "This branch does not have a committed HEAD yet, so compare-to-base is unavailable.";
    return { summary, entries: [] };
  }
  try {
    const { stdout } = await git(["rev-parse", "--verify", baseRef], worktreePath);
    baseOid = stdout.trim();
    summary.baseOid = baseOid;
  } catch {
    summary.status = "invalid-base";
    summary.errorMessage = `Base ref ${baseRef} could not be resolved in this repository.`;
    return { summary, entries: [] };
  }
  let mergeBase;
  try {
    const { stdout } = await git(["merge-base", baseOid, headOid], worktreePath);
    mergeBase = stdout.trim();
    summary.mergeBase = mergeBase;
  } catch {
    summary.status = "no-merge-base";
    summary.errorMessage = `This branch and ${baseRef} do not share a merge base, so compare-to-base is unavailable.`;
    return { summary, entries: [] };
  }
  try {
    const entries = await loadBranchChanges(mergeBase, headOid);
    const { stdout: countOut } = await git(
      ["rev-list", "--count", `${baseOid}..${headOid}`],
      worktreePath
    );
    summary.changedFiles = entries.length;
    summary.commitsAhead = Number.parseInt(countOut.trim(), 10) || 0;
    summary.status = "ready";
    return { summary, entries };
  } catch (error) {
    summary.status = "error";
    summary.errorMessage = error instanceof Error ? error.message : "Failed to load branch compare";
    return { summary, entries: [] };
  }
}
async function branchDiffEntries(git, gitBuffer, worktreePath, baseRef, opts) {
  let headOid;
  let mergeBase;
  try {
    const { stdout: headOut } = await git(["rev-parse", "--verify", "HEAD"], worktreePath);
    headOid = headOut.trim();
    const { stdout: baseOut } = await git(["rev-parse", "--verify", baseRef], worktreePath);
    const baseOid = baseOut.trim();
    const { stdout: mbOut } = await git(["merge-base", baseOid, headOid], worktreePath);
    mergeBase = mbOut.trim();
  } catch {
    return [];
  }
  const { stdout } = await git(
    ["-c", "core.quotePath=false", "diff", "--name-status", "-M", "-C", mergeBase, headOid],
    worktreePath
  );
  const allChanges = parseBranchDiff(stdout);
  let changes = allChanges;
  if (opts.filePath) {
    changes = allChanges.filter(
      (c) => c.path === opts.filePath || c.oldPath === opts.filePath || opts.oldPath && (c.path === opts.oldPath || c.oldPath === opts.oldPath)
    );
  }
  if (!opts.includePatch) {
    return changes.map(() => ({
      kind: "text",
      originalContent: "",
      modifiedContent: "",
      originalIsBinary: false,
      modifiedIsBinary: false
    }));
  }
  const results = [];
  for (const change of changes) {
    const fp = change.path;
    const oldP = change.oldPath ?? fp;
    try {
      const left = await readBlobAtOid(gitBuffer, worktreePath, mergeBase, oldP);
      const right = await readBlobAtOid(gitBuffer, worktreePath, headOid, fp);
      results.push(buildDiffResult(left.content, right.content, left.isBinary, right.isBinary, fp));
    } catch {
      results.push({
        kind: "text",
        originalContent: "",
        modifiedContent: "",
        originalIsBinary: false,
        modifiedIsBinary: false
      });
    }
  }
  return results;
}
var path4;
var init_git_handler_ops = __esm({
  "src/relay/git-handler-ops.ts"() {
    "use strict";
    path4 = __toESM(require("node:path"));
    init_git_handler_utils();
    init_git_diff_result();
    init_git_buffer_overflow();
    init_git_working_file_read();
    init_git_exec_validator();
  }
});

// src/relay/git-handler-submodule-ops.ts
function createSubmodulePathsCache() {
  return { entries: /* @__PURE__ */ new Map(), generation: 0 };
}
function clearSubmodulePathsCache(cache) {
  cache.entries.clear();
  cache.generation += 1;
}
function getCachedSubmodulePaths(cache, worktreePath, now) {
  const cached = cache.entries.get(worktreePath);
  if (!cached) {
    return null;
  }
  if (cached.expiresAt <= now) {
    cache.entries.delete(worktreePath);
    return null;
  }
  cache.entries.delete(worktreePath);
  cache.entries.set(worktreePath, cached);
  return cached.paths;
}
function pruneExpiredSubmodulePaths(cache, now) {
  for (const [worktreePath, entry] of cache.entries) {
    if (entry.expiresAt <= now) {
      cache.entries.delete(worktreePath);
    }
  }
}
function rememberSubmodulePaths(cache, worktreePath, paths, now) {
  cache.entries.delete(worktreePath);
  cache.entries.set(worktreePath, { paths, expiresAt: now + SUBMODULE_PATHS_CACHE_TTL_MS });
  while (cache.entries.size > MAX_SUBMODULE_PATHS_CACHE_ENTRIES) {
    const oldestPath = cache.entries.keys().next().value;
    if (oldestPath === void 0) {
      break;
    }
    cache.entries.delete(oldestPath);
  }
}
async function listSubmodulePathsCached(git, worktreePath, cache, now = Date.now()) {
  const cached = getCachedSubmodulePaths(cache, worktreePath, now);
  if (cached) {
    return cached;
  }
  pruneExpiredSubmodulePaths(cache, now);
  const cacheGeneration = cache.generation;
  const paths = await listSubmodulePaths(git, worktreePath);
  if (cacheGeneration === cache.generation) {
    rememberSubmodulePaths(cache, worktreePath, paths, now);
  }
  return paths;
}
async function listSubmodulePaths(git, worktreePath) {
  try {
    const { stdout } = await git(
      ["config", "--file", ".gitmodules", "--get-regexp", "^submodule\\..*\\.path$"],
      worktreePath
    );
    return stdout.split(/\r?\n/).map((line) => {
      const spaceIndex = line.indexOf(" ");
      return spaceIndex === -1 ? "" : line.slice(spaceIndex + 1).trim().replace(/\/+$/, "");
    }).filter((value) => value.length > 0);
  } catch {
    return [];
  }
}
function findContainingSubmodule(submodulePaths, filePath) {
  const normalized = filePath.replace(/\\/g, "/").replace(/\/+$/, "");
  let best = null;
  for (const sub of submodulePaths) {
    if (normalized === sub || normalized.startsWith(`${sub}/`)) {
      if (!best || sub.length > best.length) {
        best = sub;
      }
    }
  }
  return best;
}
function resolveSubmoduleWorktreePath(worktreePath, submodulePath) {
  if (!submodulePath || submodulePath.includes("\0") || path5.isAbsolute(submodulePath)) {
    throw new Error("Access denied: invalid submodule path");
  }
  const resolved = path5.resolve(worktreePath, submodulePath);
  const rel = path5.relative(path5.resolve(worktreePath), resolved);
  if (!rel || rel === ".." || rel.startsWith(`..${path5.sep}`) || path5.isAbsolute(rel)) {
    throw new Error("Access denied: submodule path resolves outside the worktree");
  }
  return resolved;
}
async function readGitlinkOidFromTree(git, worktreePath, ref, submodulePath) {
  try {
    const { stdout } = await git(["ls-tree", ref, "--", submodulePath], worktreePath);
    return stdout.match(/^160000 commit ([0-9a-f]+)\t/m)?.[1] ?? "";
  } catch {
    return "";
  }
}
async function readGitlinkOidFromIndex(git, worktreePath, submodulePath) {
  try {
    const { stdout } = await git(["ls-files", "-s", "--", submodulePath], worktreePath);
    return stdout.match(/^160000 ([0-9a-f]+) /m)?.[1] ?? "";
  } catch {
    return "";
  }
}
async function readWorkingSubmoduleHead(git, submoduleWorktreePath) {
  try {
    const { stdout } = await git(["rev-parse", "HEAD"], submoduleWorktreePath);
    return stdout.trim();
  } catch {
    return "";
  }
}
async function resolveSubmoduleCommitRange(git, worktreePath, submodulePath, staged = false) {
  const submoduleWorktreePath = resolveSubmoduleWorktreePath(worktreePath, submodulePath);
  const fromOid = staged ? await readGitlinkOidFromTree(git, worktreePath, "HEAD", submodulePath) : await readGitlinkOidFromIndex(git, worktreePath, submodulePath) || await readGitlinkOidFromTree(git, worktreePath, "HEAD", submodulePath);
  const toOid = staged ? await readGitlinkOidFromIndex(git, worktreePath, submodulePath) : await readWorkingSubmoduleHead(git, submoduleWorktreePath);
  return { fromOid, toOid };
}
async function computeSubmoduleRangeEntries(git, submoduleWorktreePath, fromOid, toOid) {
  let nameStatus = "";
  let numstat = "";
  try {
    const [statusResult, numstatResult] = await Promise.all([
      git(
        ["-c", "core.quotePath=false", "diff", "--name-status", "-M", "-C", fromOid, toOid],
        submoduleWorktreePath
      ),
      git(
        ["-c", "core.quotePath=false", "diff", "-z", "--numstat", "-M", "-C", fromOid, toOid],
        submoduleWorktreePath
      )
    ]);
    nameStatus = statusResult.stdout;
    numstat = numstatResult.stdout;
  } catch {
    return [];
  }
  return parseBranchDiff(nameStatus, parseNumstat(numstat)).map((entry) => ({
    ...entry,
    area: "unstaged"
  }));
}
async function buildSubmoduleInnerCommitRangeDiff(gitBuffer, submoduleWorktreePath, innerPath, fromOid, toOid) {
  const left = await readBlobAtOid(gitBuffer, submoduleWorktreePath, fromOid, innerPath);
  const right = await readBlobAtOid(gitBuffer, submoduleWorktreePath, toOid, innerPath);
  return buildDiffResult(left.content, right.content, left.isBinary, right.isBinary, innerPath);
}
async function computeSubmodulePointerDiff(git, worktreePath, submodulePath, staged, compareAgainstHead = false) {
  const submoduleWorktreePath = resolveSubmoduleWorktreePath(worktreePath, submodulePath);
  let leftOid = "";
  let rightOid = "";
  if (staged) {
    leftOid = await readGitlinkOidFromTree(git, worktreePath, "HEAD", submodulePath);
    rightOid = await readGitlinkOidFromIndex(git, worktreePath, submodulePath);
  } else if (compareAgainstHead) {
    leftOid = await readGitlinkOidFromTree(git, worktreePath, "HEAD", submodulePath);
    rightOid = await readWorkingSubmoduleHead(git, submoduleWorktreePath);
  } else {
    leftOid = await readGitlinkOidFromIndex(git, worktreePath, submodulePath) || await readGitlinkOidFromTree(git, worktreePath, "HEAD", submodulePath);
    rightOid = await readWorkingSubmoduleHead(git, submoduleWorktreePath);
  }
  return buildDiffResult(
    leftOid ? `Subproject commit ${leftOid}
` : "",
    rightOid ? `Subproject commit ${rightOid}
` : "",
    false,
    false,
    submodulePath
  );
}
var path5, SUBMODULE_PATHS_CACHE_TTL_MS, MAX_SUBMODULE_PATHS_CACHE_ENTRIES;
var init_git_handler_submodule_ops = __esm({
  "src/relay/git-handler-submodule-ops.ts"() {
    "use strict";
    path5 = __toESM(require("node:path"));
    init_git_diff_result();
    init_git_handler_utils();
    init_git_uncommitted_line_stats();
    init_git_handler_ops();
    SUBMODULE_PATHS_CACHE_TTL_MS = 5e3;
    MAX_SUBMODULE_PATHS_CACHE_ENTRIES = 512;
  }
});

// src/shared/process-output-field-scanner.ts
function* iterateProcessOutputLines(output) {
  let lineStart = 0;
  for (let index = 0; index < output.length; index += 1) {
    const code = output.charCodeAt(index);
    if (code !== 10 && code !== 13) {
      continue;
    }
    yield output.slice(lineStart, index);
    if (code === 13 && output.charCodeAt(index + 1) === 10) {
      index += 1;
    }
    lineStart = index + 1;
  }
  if (lineStart < output.length) {
    yield output.slice(lineStart);
  }
}
function getProcessOutputFields(line, maxFields) {
  if (maxFields <= 0) {
    return [];
  }
  const fields = [];
  const scanLimit = Math.min(line.length, PROCESS_OUTPUT_FIELD_SCAN_MAX_CHARS);
  let tokenStart = -1;
  for (let index = 0; index <= scanLimit; index += 1) {
    const isEnd = index === scanLimit;
    if (!isEnd && !isProcessOutputWhitespace(line.charCodeAt(index))) {
      if (tokenStart === -1) {
        tokenStart = index;
      }
      continue;
    }
    if (tokenStart === -1) {
      continue;
    }
    fields.push(line.slice(tokenStart, index));
    tokenStart = -1;
    if (fields.length >= maxFields) {
      break;
    }
  }
  return fields;
}
function isProcessOutputWhitespace(code) {
  return code === 32 || code >= 9 && code <= 13 || code === 160 || code === 5760 || code >= 8192 && code <= 8202 || code === 8232 || code === 8233 || code === 8239 || code === 8287 || code === 12288 || code === 65279;
}
var PROCESS_OUTPUT_FIELD_SCAN_MAX_CHARS;
var init_process_output_field_scanner = __esm({
  "src/shared/process-output-field-scanner.ts"() {
    "use strict";
    PROCESS_OUTPUT_FIELD_SCAN_MAX_CHARS = 4096;
  }
});

// src/shared/git-rev-list-output.ts
function parseGitRevListAheadBehindCounts(output) {
  const fields = getProcessOutputFields(output, 3);
  if (fields.length !== 2) {
    return { status: "unexpected-field-count" };
  }
  const ahead = parseGitRevListNonNegativeCount(fields[0]);
  const behind = parseGitRevListNonNegativeCount(fields[1]);
  if (ahead === null || behind === null) {
    return { status: "unparseable-counts" };
  }
  return { status: "ok", ahead, behind };
}
function parseGitRevListFirstParentOid(output) {
  return getProcessOutputFields(output, 2)[1] ?? null;
}
function parseGitRevListNonNegativeCount(value) {
  if (!value || !/^\d+$/.test(value)) {
    return null;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isSafeInteger(parsed) ? parsed : null;
}
var init_git_rev_list_output = __esm({
  "src/shared/git-rev-list-output.ts"() {
    "use strict";
    init_process_output_field_scanner();
  }
});

// src/relay/git-handler-commit-diff-ops.ts
function assertFullGitObjectId(value, label) {
  if (!FULL_GIT_OBJECT_ID_PATTERN.test(value)) {
    throw new Error(`${label} must be a full git object id`);
  }
}
async function commitCompare(git, worktreePath, commitId) {
  assertFullGitObjectId(commitId, "commitId");
  let commitOid = "";
  try {
    const { stdout } = await git(
      ["rev-parse", "--verify", "--end-of-options", `${commitId}^{commit}`],
      worktreePath
    );
    commitOid = stdout.trim();
  } catch {
    return {
      summary: {
        commitOid: "",
        parentOid: null,
        compareRef: commitId,
        baseRef: "parent",
        changedFiles: 0,
        status: "invalid-commit",
        errorMessage: `Commit ${commitId} could not be resolved in this repository.`
      },
      entries: []
    };
  }
  const summary = {
    commitOid,
    parentOid: null,
    compareRef: commitOid.slice(0, 7),
    baseRef: "empty tree",
    changedFiles: 0,
    status: "ready"
  };
  try {
    const { stdout: parentsOut } = await git(
      ["rev-list", "--parents", "-n", "1", commitOid],
      worktreePath
    );
    const firstParent = parseGitRevListFirstParentOid(parentsOut);
    summary.parentOid = firstParent;
    summary.baseRef = firstParent ? firstParent.slice(0, 7) : "empty tree";
    const diffArgs = summary.parentOid ? [
      "-c",
      "core.quotePath=false",
      "diff",
      "--name-status",
      "-M",
      "-C",
      summary.parentOid,
      commitOid
    ] : [
      "-c",
      "core.quotePath=false",
      "diff-tree",
      "--root",
      "--no-commit-id",
      "--name-status",
      "-r",
      "-M",
      "-C",
      commitOid
    ];
    const numstatArgs = summary.parentOid ? [
      "-c",
      "core.quotePath=false",
      "diff",
      "--numstat",
      "-M",
      "-C",
      summary.parentOid,
      commitOid
    ] : [
      "-c",
      "core.quotePath=false",
      "diff-tree",
      "--root",
      "--no-commit-id",
      "--numstat",
      "-r",
      "-M",
      "-C",
      commitOid
    ];
    const [{ stdout }, { stdout: numstat }] = await Promise.all([
      git(diffArgs, worktreePath),
      git(numstatArgs, worktreePath)
    ]);
    const entries = parseBranchDiff(stdout, parseNumstat(numstat));
    summary.changedFiles = entries.length;
    return { summary, entries };
  } catch (error) {
    return {
      summary: {
        ...summary,
        status: "error",
        errorMessage: error instanceof Error ? error.message : "Failed to load commit diff"
      },
      entries: []
    };
  }
}
async function commitDiffEntry(gitBuffer, worktreePath, args) {
  assertFullGitObjectId(args.commitOid, "commitOid");
  if (args.parentOid) {
    assertFullGitObjectId(args.parentOid, "parentOid");
  }
  try {
    const oldPath = args.oldPath ?? args.filePath;
    const left = args.parentOid ? await readBlobAtOid(gitBuffer, worktreePath, args.parentOid, oldPath) : { content: "", isBinary: false };
    const right = await readBlobAtOid(gitBuffer, worktreePath, args.commitOid, args.filePath);
    return buildDiffResult(
      left.content,
      right.content,
      left.isBinary,
      right.isBinary,
      args.filePath
    );
  } catch {
    return {
      kind: "text",
      originalContent: "",
      modifiedContent: "",
      originalIsBinary: false,
      modifiedIsBinary: false
    };
  }
}
var FULL_GIT_OBJECT_ID_PATTERN;
var init_git_handler_commit_diff_ops = __esm({
  "src/relay/git-handler-commit-diff-ops.ts"() {
    "use strict";
    init_git_handler_ops();
    init_git_handler_utils();
    init_git_diff_result();
    init_git_uncommitted_line_stats();
    init_git_rev_list_output();
    FULL_GIT_OBJECT_ID_PATTERN = /^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$/;
  }
});

// src/shared/worktree-base-ref.ts
async function resolveWorktreeAddBaseRef(baseRef, refExists) {
  if (baseRef.startsWith("refs/")) {
    return baseRef;
  }
  const candidates = baseRef.includes("/") ? [`refs/remotes/${baseRef}`, `refs/heads/${baseRef}`] : [`refs/heads/${baseRef}`];
  for (const candidate of candidates) {
    if (await refExists(candidate)) {
      return candidate;
    }
  }
  return baseRef;
}
var init_worktree_base_ref = __esm({
  "src/shared/worktree-base-ref.ts"() {
    "use strict";
  }
});

// src/shared/worktree-removal.ts
function createLockedWorktreeRemovalError(lockReason) {
  const reason = lockReason?.trim();
  return new Error(
    reason ? `${LOCKED_WORKTREE_REMOVAL_PREFIX} Lock reason: ${reason}. Run git worktree unlock <worktree-path> from its repository, then retry deletion.` : `${LOCKED_WORKTREE_REMOVAL_PREFIX} Run git worktree unlock <worktree-path> from its repository, then retry deletion.`
  );
}
function assertWorktreeUnlockedForRemoval(worktree) {
  if (worktree?.locked) {
    throw createLockedWorktreeRemovalError(worktree.lockReason);
  }
}
var LOCKED_WORKTREE_REMOVAL_PREFIX;
var init_worktree_removal = __esm({
  "src/shared/worktree-removal.ts"() {
    "use strict";
    LOCKED_WORKTREE_REMOVAL_PREFIX = "Worktree is locked by Git.";
  }
});

// src/shared/git-merge-tree-capability.ts
function getGitErrorText2(error) {
  if (typeof error !== "object" || error === null) {
    return error instanceof Error ? error.message : String(error);
  }
  const values = ["message", "stderr", "stdout"].map((key) => error[key]).filter((value) => typeof value === "string");
  return values.join("\n");
}
function isUnsupportedMergeTreeWriteTreeError(error) {
  const output = getGitErrorText2(error);
  return /(?:unknown|invalid|unrecognized) option(?::|\s+)[`']?(?:--?)?write-tree[`']?(?:\s|$)/i.test(
    output
  ) || /unknown rev [`']?--write-tree[`']?(?:\s|$)/i.test(output) || /usage:\s*git merge-tree\s+<base-tree>\s+<branch1>\s+<branch2>/i.test(output);
}
var init_git_merge_tree_capability = __esm({
  "src/shared/git-merge-tree-capability.ts"() {
    "use strict";
  }
});

// src/shared/git-branch-cleanup.ts
async function readOptionalGitStdout(runGit, argv, options) {
  try {
    const { stdout } = await runGit(argv, options);
    return stdout.trim() || null;
  } catch {
    return null;
  }
}
async function readOptionalGitRawStdout(runGit, argv) {
  try {
    const { stdout } = await runGit(argv);
    return stdout || null;
  } catch {
    return null;
  }
}
function addCandidateRef(candidates, ref) {
  const trimmed = ref?.trim();
  if (!trimmed || trimmed.startsWith("-") || candidates.includes(trimmed)) {
    return;
  }
  candidates.push(trimmed);
}
async function getBranchCleanupTargetRefs(runGit, branchName) {
  const candidates = [];
  addCandidateRef(
    candidates,
    await readOptionalGitStdout(runGit, ["config", "--get", `branch.${branchName}.base`])
  );
  addCandidateRef(
    candidates,
    await readOptionalGitStdout(runGit, ["symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"])
  );
  addCandidateRef(candidates, "HEAD");
  return candidates;
}
async function refreshBranchCleanupTargetRefs(runGit, targetRefs) {
  const remotesStdout = await readOptionalGitStdout(runGit, ["remote"]);
  const remotes = (remotesStdout ?? "").split(/\r?\n/).map((line) => line.trim()).filter((remote) => remote && !remote.startsWith("-")).sort((left, right) => right.length - left.length);
  const fetchedRemotes = /* @__PURE__ */ new Set();
  for (const targetRef of targetRefs) {
    const remote = remotes.find((candidate) => targetRef.startsWith(`refs/remotes/${candidate}/`));
    if (!remote || fetchedRemotes.has(remote)) {
      continue;
    }
    fetchedRemotes.add(remote);
    await readOptionalGitStdout(runGit, ["fetch", "--prune", remote]);
  }
}
async function resolveCommitOid(runGit, ref) {
  return readOptionalGitStdout(runGit, ["rev-parse", "--verify", "--quiet", `${ref}^{commit}`]);
}
async function hasBranchOnlyMergeCommits(runGit, targetOid, branchRef) {
  const stdout = await readOptionalGitStdout(runGit, [
    "rev-list",
    "--right-only",
    "--merges",
    "--count",
    `${targetOid}...${branchRef}`
  ]);
  return Number(stdout ?? 0) > 0;
}
async function branchMergesWithoutTreeChanges(runGit, targetOid, branchRef, capabilities) {
  const args = ["merge-tree", "--write-tree", targetOid, branchRef];
  const readMergedTree = async () => {
    try {
      return await capabilities.runWithFallback(
        "merge-tree-write-tree",
        async () => (await runGit(args)).stdout.trim() || null,
        async () => null,
        isUnsupportedMergeTreeWriteTreeError
      );
    } catch {
      return null;
    }
  };
  const mergedTree = await readMergedTree();
  if (!mergedTree) {
    return false;
  }
  const targetTree = await readOptionalGitStdout(runGit, [
    "rev-parse",
    "--verify",
    "--quiet",
    `${targetOid}^{tree}`
  ]);
  return Boolean(mergedTree && targetTree && mergedTree.split(/\r?\n/)[0] === targetTree);
}
async function branchOnlyCommitsArePatchEquivalent(runGit, targetOid, branchRef) {
  const stdout = await readOptionalGitStdout(runGit, ["cherry", "-v", targetOid, branchRef]);
  if (stdout === null) {
    return false;
  }
  const lines = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  return lines.every((line) => line.startsWith("-"));
}
function parsePatchId(stdout) {
  const line = stdout?.split(/\r?\n/).map((candidate) => candidate.trim()).find(Boolean);
  const patchId = line?.split(/\s+/)[0];
  return patchId || null;
}
async function computeStablePatchId(runGit, patchText) {
  if (!patchText) {
    return null;
  }
  return parsePatchId(
    await readOptionalGitStdout(runGit, ["patch-id", "--stable"], { stdin: patchText })
  );
}
async function branchNetPatchMatchesTargetSquashCommit(runGit, targetOid, branchRef, capabilities) {
  const mergeBase = await readOptionalGitStdout(runGit, ["merge-base", targetOid, branchRef]);
  if (!mergeBase) {
    return false;
  }
  const branchPatchId = await computeStablePatchId(
    runGit,
    await readOptionalGitRawStdout(runGit, ["diff", mergeBase, branchRef])
  );
  if (!branchPatchId) {
    return false;
  }
  const commits = (await readOptionalGitStdout(runGit, [
    "rev-list",
    "--ancestry-path",
    `--max-count=${SQUASH_PATCH_SCAN_LIMIT + 1}`,
    `${mergeBase}..${targetOid}`
  ]))?.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  if (!commits?.length || commits.length > SQUASH_PATCH_SCAN_LIMIT) {
    return false;
  }
  for (const commitOid of commits) {
    const commitPatchId = await computeStablePatchId(
      runGit,
      await readOptionalGitRawStdout(runGit, ["show", "--format=", commitOid])
    );
    if (commitPatchId === branchPatchId && await branchMergesWithoutTreeChanges(runGit, commitOid, branchRef, capabilities)) {
      return true;
    }
  }
  return false;
}
async function branchHasNoUnmergedChangesOnAnyTarget(runGit, branchName, targetRefs, capabilities) {
  const branchRef = `refs/heads/${branchName}`;
  for (const targetRef of targetRefs) {
    const targetOid = await resolveCommitOid(runGit, targetRef);
    if (!targetOid) {
      continue;
    }
    if (await branchMergesWithoutTreeChanges(runGit, targetOid, branchRef, capabilities)) {
      return true;
    }
    if (await hasBranchOnlyMergeCommits(runGit, targetOid, branchRef)) {
      if (await branchNetPatchMatchesTargetSquashCommit(runGit, targetOid, branchRef, capabilities)) {
        return true;
      }
      continue;
    }
    if (await branchOnlyCommitsArePatchEquivalent(runGit, targetOid, branchRef)) {
      return true;
    }
  }
  return false;
}
var SQUASH_PATCH_SCAN_LIMIT;
var init_git_branch_cleanup = __esm({
  "src/shared/git-branch-cleanup.ts"() {
    "use strict";
    init_git_merge_tree_capability();
    SQUASH_PATCH_SCAN_LIMIT = 200;
  }
});

// src/relay/git-handler-branch-cleanup.ts
async function deleteAlreadyMergedRelayBranchAfterSafeDeleteFailure(git, repoPath, branchName, branchHead, capabilities) {
  const runGit = (args, options) => options ? git(args, repoPath, options) : git(args, repoPath);
  const targetRefs = await getBranchCleanupTargetRefs(runGit, branchName);
  await refreshBranchCleanupTargetRefs(runGit, targetRefs);
  if (!await branchHasNoUnmergedChangesOnAnyTarget(runGit, branchName, targetRefs, capabilities)) {
    return false;
  }
  await deleteRelayBranchAtExpectedHead(git, repoPath, branchName, branchHead);
  return true;
}
async function forceDeletePreservedRelayBranch(git, repoPath, branchName, expectedHead) {
  if (!branchName || branchName.includes("\0") || branchName.startsWith("-")) {
    throw new Error("Invalid branch name for preserved branch delete.");
  }
  if (!expectedHead) {
    throw new Error("Expected branch head is required for preserved branch delete.");
  }
  await deleteRelayBranchAtExpectedHead(git, repoPath, branchName, expectedHead, () => {
    return new Error(
      `Local branch "${branchName}" changed after the workspace was deleted. Review it before deleting it.`
    );
  });
}
async function deleteRelayBranchAtExpectedHead(git, repoPath, branchName, expectedHead, mapUpdateRefError) {
  if (await isRelayBranchCheckedOut(git, repoPath, branchName)) {
    throw new Error(`Local branch "${branchName}" is checked out in another worktree.`);
  }
  try {
    await git(["update-ref", "-d", `refs/heads/${branchName}`, expectedHead], repoPath);
  } catch (error) {
    throw mapUpdateRefError?.(error) ?? error;
  }
  if (await isRelayBranchCheckedOut(git, repoPath, branchName)) {
    try {
      await git(["update-ref", `refs/heads/${branchName}`, expectedHead, ""], repoPath);
    } catch (restoreError) {
      console.warn(
        `relay removeWorktree: failed to restore local branch "${branchName}" after concurrent checkout`,
        restoreError
      );
    }
    throw new Error(`Local branch "${branchName}" is checked out in another worktree.`);
  }
  try {
    await git(["config", "--remove-section", `branch.${branchName}`], repoPath);
  } catch {
  }
}
async function isRelayBranchCheckedOut(git, repoPath, branchName) {
  const { stdout } = await git(["worktree", "list", "--porcelain"], repoPath);
  return parseWorktreeList(stdout).some(
    (worktree) => typeof worktree.branch === "string" && worktree.branch.replace(/^refs\/heads\//, "") === branchName
  );
}
var init_git_handler_branch_cleanup = __esm({
  "src/relay/git-handler-branch-cleanup.ts"() {
    "use strict";
    init_git_branch_cleanup();
    init_git_handler_utils();
  }
});

// src/relay/git-handler-worktree-list.ts
async function readRelayWorktreeList(git, repoPath, capabilities) {
  return capabilities.runWithFallback(
    "worktree-list-z",
    async () => {
      const { stdout } = await git(["worktree", "list", "--porcelain", "-z"], repoPath);
      return normalizeRelayWorktrees(parseWorktreeList(stdout, { nulDelimited: true }));
    },
    async () => {
      const { stdout } = await git(["worktree", "list", "--porcelain"], repoPath);
      return normalizeRelayWorktrees(parseWorktreeList(stdout));
    },
    isUnsupportedWorktreeListZError
  );
}
function normalizeRelayWorktrees(worktrees) {
  return worktrees.map((worktree) => ({
    path: typeof worktree.path === "string" ? worktree.path : "",
    head: typeof worktree.head === "string" ? worktree.head : void 0,
    branch: typeof worktree.branch === "string" ? worktree.branch : void 0,
    locked: worktree.locked === true ? true : void 0,
    lockReason: typeof worktree.lockReason === "string" ? worktree.lockReason : void 0
  })).filter((worktree) => worktree.path.length > 0);
}
var init_git_handler_worktree_list = __esm({
  "src/relay/git-handler-worktree-list.ts"() {
    "use strict";
    init_git_handler_utils();
  }
});

// src/relay/git-handler-worktree-remove.ts
function getErrorText(error) {
  if (typeof error === "object" && error !== null) {
    const parts = [];
    if ("message" in error && typeof error.message === "string") {
      parts.push(error.message);
    }
    if ("stderr" in error && typeof error.stderr === "string") {
      parts.push(error.stderr);
    }
    if ("stdout" in error && typeof error.stdout === "string") {
      parts.push(error.stdout);
    }
    return parts.join("\n");
  }
  return String(error);
}
function isBranchCheckedOutInWorktreeError(error) {
  return /cannot delete branch .*(?:used by worktree|checked out)|branch .*is checked out/i.test(
    getErrorText(error)
  );
}
function normalizeLocalBranchRef(branch) {
  return branch.replace(/^refs\/heads\//, "");
}
function isPosixAbsolutePath(value) {
  return value.startsWith("/");
}
function isWindowsAbsolutePath(value) {
  return /^[A-Za-z]:[\\/]/.test(value) || value.startsWith("\\\\");
}
function resolveRelayRepoPath(worktreePath, commonDir) {
  if (isPosixAbsolutePath(worktreePath) || isPosixAbsolutePath(commonDir)) {
    return path6.posix.resolve(worktreePath, commonDir, "..");
  }
  if (isWindowsAbsolutePath(worktreePath) || isWindowsAbsolutePath(commonDir)) {
    return path6.win32.resolve(worktreePath, commonDir, "..");
  }
  return path6.resolve(worktreePath, commonDir, "..");
}
function normalizeRelayWorktreePathForCompare(value) {
  if (isPosixAbsolutePath(value)) {
    return path6.posix.normalize(path6.posix.resolve(value));
  }
  if (isWindowsAbsolutePath(value)) {
    return path6.win32.normalize(path6.win32.resolve(value));
  }
  return path6.normalize(path6.resolve(value));
}
function areRelayWorktreePathsEqual(leftPath, rightPath) {
  const left = normalizeRelayWorktreePathForCompare(leftPath);
  const right = normalizeRelayWorktreePathForCompare(rightPath);
  const compareCaseInsensitive = isWindowsAbsolutePath(leftPath) && isWindowsAbsolutePath(rightPath);
  return compareCaseInsensitive ? left.toLowerCase() === right.toLowerCase() : left === right;
}
async function listRelayWorktreesForRemoval(git, repoPath, capabilities) {
  try {
    return await readRelayWorktreeList(git, repoPath, capabilities);
  } catch {
    return [];
  }
}
async function deleteRelayBranchAfterWorktreeRemoval(git, repoPath, branchName, forceBranchDelete) {
  const deleteFlag = forceBranchDelete ? "-D" : "-d";
  try {
    await git(["branch", deleteFlag, "--", branchName], repoPath);
    return "deleted";
  } catch (error) {
    if (!isBranchCheckedOutInWorktreeError(error)) {
      throw error;
    }
  }
  try {
    await git(["worktree", "prune"], repoPath);
  } catch (error) {
    console.warn(
      `relay removeWorktree: failed to prune worktrees before deleting branch "${branchName}"`,
      error
    );
    return "checked-out";
  }
  try {
    await git(["branch", deleteFlag, "--", branchName], repoPath);
    return "deleted";
  } catch (error) {
    if (isBranchCheckedOutInWorktreeError(error)) {
      return "checked-out";
    }
    throw error;
  }
}
async function removeWorktreeOp(git, params, capabilities) {
  const worktreePath = params.worktreePath;
  const force = params.force;
  const deleteBranch = params.deleteBranch !== false;
  const forceBranchDelete = params.forceBranchDelete === true;
  let repoPath = worktreePath;
  try {
    const { stdout } = await git(["rev-parse", "--git-common-dir"], worktreePath);
    const commonDir = stdout.trim();
    if (commonDir && commonDir !== ".git") {
      repoPath = resolveRelayRepoPath(worktreePath, commonDir);
    }
  } catch {
  }
  const worktreesBeforeRemoval = await listRelayWorktreesForRemoval(git, repoPath, capabilities);
  const removedWorktree = worktreesBeforeRemoval.find(
    (worktree) => areRelayWorktreePathsEqual(worktree.path, worktreePath)
  );
  const branchName = normalizeLocalBranchRef(removedWorktree?.branch ?? "");
  const branchHead = removedWorktree?.head ?? "";
  assertWorktreeUnlockedForRemoval(removedWorktree);
  const args = ["worktree", "remove"];
  if (force) {
    args.push("--force");
  }
  args.push(worktreePath);
  await git(args, repoPath);
  if (!branchName) {
    return {};
  }
  if (!deleteBranch) {
    return {};
  }
  try {
    const branchDeleteResult = await deleteRelayBranchAfterWorktreeRemoval(
      git,
      repoPath,
      branchName,
      forceBranchDelete
    );
    if (branchDeleteResult === "checked-out") {
      return {};
    }
    return {};
  } catch (error) {
    if (!forceBranchDelete && branchHead) {
      try {
        if (await deleteAlreadyMergedRelayBranchAfterSafeDeleteFailure(
          git,
          repoPath,
          branchName,
          branchHead,
          capabilities
        )) {
          return {};
        }
      } catch (alreadyMergedDeleteError) {
        console.warn(
          `relay removeWorktree: failed to delete already-merged local branch "${branchName}" after removing worktree`,
          alreadyMergedDeleteError
        );
      }
    }
    console.warn(
      `relay removeWorktree: preserved local branch "${branchName}" after removing worktree (not fully merged)`,
      error
    );
    return { preservedBranch: { branchName, ...branchHead ? { head: branchHead } : {} } };
  }
}
var path6;
var init_git_handler_worktree_remove = __esm({
  "src/relay/git-handler-worktree-remove.ts"() {
    "use strict";
    path6 = __toESM(require("node:path"));
    init_worktree_removal();
    init_git_handler_branch_cleanup();
    init_git_handler_worktree_list();
  }
});

// src/relay/git-handler-worktree-ops.ts
async function persistRelayWorktreeCreationBase(git, targetDir, branchName, effectiveBase) {
  const configKey = `branch.${branchName}.base`;
  try {
    await git(["config", "--local", "--replace-all", configKey, effectiveBase], targetDir);
  } catch (error) {
    console.warn(`relay addWorktree: failed to set ${configKey} for ${targetDir}`, error);
    try {
      await git(["config", "--local", "--unset-all", configKey], targetDir);
    } catch (unsetError) {
      console.warn(
        `relay addWorktree: failed to unset stale ${configKey} for ${targetDir}`,
        unsetError
      );
    }
  }
}
async function addWorktreeOp(git, params) {
  const repoPath = params.repoPath;
  const branchName = params.branchName;
  const targetDir = params.targetDir;
  const base = params.base;
  const checkoutExistingBranch = params.checkoutExistingBranch === true;
  const noCheckout = params.noCheckout === true;
  if (branchName.startsWith("-") || base && base.startsWith("-")) {
    throw new Error('Branch name and base ref must not start with "-"');
  }
  const effectiveBase = base && !checkoutExistingBranch ? await resolveWorktreeAddBaseRef(base, async (qualifiedRef) => {
    try {
      await git(["rev-parse", "--verify", "--quiet", `${qualifiedRef}^{commit}`], repoPath);
      return true;
    } catch {
      return false;
    }
  }) : void 0;
  const args = checkoutExistingBranch ? ["worktree", "add", targetDir, branchName] : ["worktree", "add", "--no-track", "-b", branchName, targetDir];
  if (!checkoutExistingBranch && noCheckout) {
    args.splice(3, 0, "--no-checkout");
  }
  if (effectiveBase) {
    args.push(effectiveBase);
  }
  await git(args, repoPath);
  if (checkoutExistingBranch) {
    return;
  }
  if (effectiveBase) {
    await persistRelayWorktreeCreationBase(git, targetDir, branchName, effectiveBase);
  }
  try {
    let alreadySet = false;
    try {
      await git(["config", "--get", "push.autoSetupRemote"], targetDir);
      alreadySet = true;
    } catch (readError) {
      const code = readError?.code;
      if (code !== 1) {
        throw readError;
      }
    }
    if (!alreadySet) {
      await git(["config", "--local", "push.autoSetupRemote", "true"], targetDir);
    }
  } catch (error) {
    console.warn(`relay addWorktree: failed to set push.autoSetupRemote for ${targetDir}`, error);
  }
}
function isPosixAbsolutePath2(value) {
  return value.startsWith("/");
}
function isWindowsAbsolutePath2(value) {
  return /^[A-Za-z]:[\\/]/.test(value) || value.startsWith("\\\\");
}
function normalizeRelayWorktreePathForCompare2(value) {
  if (isPosixAbsolutePath2(value)) {
    return path7.posix.normalize(path7.posix.resolve(value));
  }
  if (isWindowsAbsolutePath2(value)) {
    return path7.win32.normalize(path7.win32.resolve(value));
  }
  return path7.normalize(path7.resolve(value));
}
function areRelayWorktreePathsEqual2(leftPath, rightPath) {
  const left = normalizeRelayWorktreePathForCompare2(leftPath);
  const right = normalizeRelayWorktreePathForCompare2(rightPath);
  const compareCaseInsensitive = isWindowsAbsolutePath2(leftPath) && isWindowsAbsolutePath2(rightPath);
  return compareCaseInsensitive ? left.toLowerCase() === right.toLowerCase() : left === right;
}
async function worktreeIsCleanOp(git, params) {
  const worktreePath = params.worktreePath;
  const includeUntracked = params.includeUntracked !== false;
  const { stdout } = await git(
    ["status", "--porcelain", includeUntracked ? "--untracked-files=all" : "--untracked-files=no"],
    worktreePath
  );
  const clean = !stdout.trim();
  return { clean, stdout: clean ? void 0 : stdout };
}
async function commitChangesRelay(git, worktreePath, message) {
  if (typeof message !== "string" || message.trim().length === 0) {
    return { success: false, error: "Commit message is required" };
  }
  try {
    await git(["commit", "-m", message], worktreePath);
    return { success: true };
  } catch (error) {
    const readStringField = (field) => {
      if (typeof error === "object" && error && field in error) {
        const v = error[field];
        if (typeof v === "string" && v.length > 0) {
          return v;
        }
      }
      return null;
    };
    const errorMessage = readStringField("stderr") ?? readStringField("stdout") ?? (error instanceof Error ? error.message : "Commit failed");
    return { success: false, error: errorMessage };
  }
}
var path7;
var init_git_handler_worktree_ops = __esm({
  "src/relay/git-handler-worktree-ops.ts"() {
    "use strict";
    path7 = __toESM(require("node:path"));
    init_worktree_base_ref();
    init_git_handler_worktree_remove();
    init_git_handler_worktree_list();
  }
});

// src/relay/git-handler-local-base-ref-refresh.ts
async function refreshLocalBaseRefForWorktreeCreateOp(git, params, capabilities) {
  const repoPath = params.repoPath;
  const fullRef = params.fullRef;
  const remoteTrackingRef = params.remoteTrackingRef;
  const ownerWorktreePath = params.ownerWorktreePath;
  const checkOnly = params.checkOnly === true;
  if (typeof repoPath !== "string" || typeof fullRef !== "string" || typeof remoteTrackingRef !== "string" || ownerWorktreePath !== void 0 && typeof ownerWorktreePath !== "string") {
    throw new Error("Invalid local base ref refresh request.");
  }
  if (!fullRef.startsWith("refs/heads/") || !remoteTrackingRef.startsWith("refs/remotes/")) {
    throw new Error("Invalid local base ref refresh refs.");
  }
  await git(["check-ref-format", fullRef], repoPath);
  await git(["check-ref-format", remoteTrackingRef], repoPath);
  const localOid = await revParseCommit(git, repoPath, fullRef, "Local base ref is missing.");
  const remoteOid = await revParseCommit(
    git,
    repoPath,
    remoteTrackingRef,
    "Remote-tracking base ref is missing."
  );
  try {
    await git(["merge-base", "--is-ancestor", localOid, remoteOid], repoPath);
  } catch {
    throw new Error("Local base ref is not a fast-forward update.");
  }
  const worktrees = await readRelayWorktreeList(git, repoPath, capabilities);
  const ownerWorktree = worktrees.find((worktree) => worktree.branch === fullRef);
  if (ownerWorktree) {
    if (ownerWorktreePath && !areRelayWorktreePathsEqual2(ownerWorktree.path, ownerWorktreePath)) {
      throw new Error("Local base ref is checked out in a different worktree.");
    }
    const { stdout } = await git(
      ["status", "--porcelain", "--untracked-files=no"],
      ownerWorktree.path
    );
    if (stdout.trim()) {
      throw new Error("Local base ref worktree has tracked changes.");
    }
    if (checkOnly) {
      return;
    }
    await git(["reset", "--hard", remoteOid], ownerWorktree.path);
    return;
  }
  if (checkOnly) {
    return;
  }
  await git(["update-ref", fullRef, remoteOid, localOid], repoPath);
}
async function revParseCommit(git, repoPath, ref, missingMessage) {
  const { stdout } = await git(["rev-parse", "--verify", `${ref}^{commit}`], repoPath);
  const oid = stdout.trim();
  if (!oid) {
    throw new Error(missingMessage);
  }
  return oid;
}
var init_git_handler_local_base_ref_refresh = __esm({
  "src/relay/git-handler-local-base-ref-refresh.ts"() {
    "use strict";
    init_git_handler_worktree_ops();
  }
});

// src/shared/git-exec-mutation.ts
function gitExecMutatesRepository(args) {
  return MUTATING_GIT_EXEC_SUBCOMMANDS.has(args[0] ?? "");
}
var MUTATING_GIT_EXEC_SUBCOMMANDS;
var init_git_exec_mutation = __esm({
  "src/shared/git-exec-mutation.ts"() {
    "use strict";
    MUTATING_GIT_EXEC_SUBCOMMANDS = /* @__PURE__ */ new Set(["clone", "commit", "init"]);
  }
});

// src/relay/git-status-output-parser.ts
function parseStatusChar(char) {
  switch (char) {
    case "M":
      return "modified";
    case "A":
      return "added";
    case "D":
      return "deleted";
    case "R":
      return "renamed";
    case "C":
      return "copied";
    default:
      return "modified";
  }
}
function parseStatusOutput(stdout) {
  const entries = [];
  const unmergedLines = [];
  const ignoredPaths = [];
  let head;
  let branch;
  let upstreamName;
  let upstreamAheadBehind = null;
  for (const line of stdout.split(/\r?\n/)) {
    if (!line) {
      continue;
    }
    if (line.startsWith("# branch.oid ")) {
      head = line.slice("# branch.oid ".length).trim();
      continue;
    }
    if (line.startsWith("# branch.head ")) {
      const branchHead = line.slice("# branch.head ".length).trim();
      branch = branchHead && branchHead !== "(detached)" ? `refs/heads/${branchHead}` : "";
      continue;
    }
    if (line.startsWith("# branch.upstream ")) {
      upstreamName = line.slice("# branch.upstream ".length).trim() || void 0;
      continue;
    }
    if (line.startsWith("# branch.ab ")) {
      upstreamAheadBehind = parseBranchAheadBehind(line);
      continue;
    }
    if (line.startsWith("1 ") || line.startsWith("2 ")) {
      const parts = line.split(" ");
      const xy = parts[1];
      const indexStatus = xy[0];
      const worktreeStatus = xy[1];
      if (line.startsWith("2 ")) {
        const tabParts = line.split("	");
        const filePath = tabParts[0].split(" ").slice(9).join(" ");
        const oldPath = tabParts.slice(1).join("	");
        if (indexStatus !== ".") {
          entries.push({
            path: filePath,
            status: parseStatusChar(indexStatus),
            area: "staged",
            oldPath,
            ...submoduleStatusField(parts[2], indexStatus)
          });
        }
        if (worktreeStatus !== ".") {
          entries.push({
            path: filePath,
            status: parseStatusChar(worktreeStatus),
            area: "unstaged",
            oldPath,
            ...submoduleStatusField(parts[2], worktreeStatus)
          });
        }
      } else {
        const filePath = parts.slice(8).join(" ");
        if (indexStatus !== ".") {
          entries.push({
            path: filePath,
            status: parseStatusChar(indexStatus),
            area: "staged",
            ...submoduleStatusField(parts[2], indexStatus)
          });
        }
        if (worktreeStatus !== ".") {
          entries.push({
            path: filePath,
            status: parseStatusChar(worktreeStatus),
            area: "unstaged",
            ...submoduleStatusField(parts[2], worktreeStatus)
          });
        }
      }
    } else if (line.startsWith("? ")) {
      entries.push({ path: line.slice(2), status: "untracked", area: "untracked" });
    } else if (line.startsWith("! ")) {
      ignoredPaths.push(line.slice(2));
    } else if (line.startsWith("u ")) {
      unmergedLines.push(line);
    }
  }
  return {
    entries,
    unmergedLines,
    ignoredPaths,
    head,
    branch,
    upstreamStatus: upstreamName ? {
      hasUpstream: true,
      upstreamName,
      ahead: upstreamAheadBehind?.ahead ?? 0,
      behind: upstreamAheadBehind?.behind ?? 0
    } : { hasUpstream: false, ahead: 0, behind: 0 }
  };
}
function parseSubmoduleStatus(submoduleField, statusChar = ".") {
  if (!submoduleField?.startsWith("S")) {
    return void 0;
  }
  return {
    commitChanged: submoduleField[1] === "C" || submoduleField === "S..." && statusChar === "M",
    trackedChanges: submoduleField[2] === "M",
    untrackedChanges: submoduleField[3] === "U"
  };
}
function submoduleStatusField(submoduleField, statusChar) {
  const submodule = parseSubmoduleStatus(submoduleField, statusChar);
  return submodule ? { submodule } : {};
}
function parseBranchAheadBehind(line) {
  const match = line.match(/^# branch\.ab \+(\d+) -(\d+)$/);
  if (!match) {
    return null;
  }
  return {
    ahead: Number.parseInt(match[1], 10),
    behind: Number.parseInt(match[2], 10)
  };
}
var init_git_status_output_parser = __esm({
  "src/relay/git-status-output-parser.ts"() {
    "use strict";
  }
});

// src/shared/source-control-push-failure.ts
function normalizePushFailure(raw) {
  return raw.slice(0, PUSH_FAILURE_SUMMARY_SCAN_CODE_UNITS).replace(ANSI_PATTERN, "").replace(/\r\n?/g, "\n").replace(CONTROL_PATTERN, "").trim();
}
function isPushHookFailure(raw) {
  const normalized = normalizePushFailure(raw);
  if (!normalized) {
    return false;
  }
  if (REMOTE_PUSH_EXCLUSION_PATTERN.test(normalized)) {
    return false;
  }
  if (/hook declined to push/i.test(normalized)) {
    return true;
  }
  if (PUSH_HOOK_PATTERN.test(normalized)) {
    return true;
  }
  if (PUSH_HOOK_RUNNER_PATTERN.test(normalized) && PUSH_CONTEXT_PATTERN.test(normalized)) {
    return true;
  }
  if (LINT_PATTERN.test(normalized) && PUSH_CONTEXT_PATTERN.test(normalized)) {
    return true;
  }
  return false;
}
var PUSH_FAILURE_SUMMARY_SCAN_CODE_UNITS, ANSI_PATTERN, CONTROL_PATTERN, PUSH_HOOK_PATTERN, PUSH_HOOK_RUNNER_PATTERN, PUSH_CONTEXT_PATTERN, LINT_PATTERN, REMOTE_PUSH_EXCLUSION_PATTERN;
var init_source_control_push_failure = __esm({
  "src/shared/source-control-push-failure.ts"() {
    "use strict";
    PUSH_FAILURE_SUMMARY_SCAN_CODE_UNITS = 64 * 1024;
    ANSI_PATTERN = // eslint-disable-next-line no-control-regex
    /[\u001b\u009b][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[a-zA-Z\d]*)*)?\u0007)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PR-TZcf-nq-uy=><~]))/g;
    CONTROL_PATTERN = // eslint-disable-next-line no-control-regex
    /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g;
    PUSH_HOOK_PATTERN = /\b(?:pre-push|prepush)\b/i;
    PUSH_HOOK_RUNNER_PATTERN = /\b(?:husky|lint-staged|lefthook)\b/i;
    PUSH_CONTEXT_PATTERN = /\b(?:failed to push|hook declined to push|git push)\b/i;
    LINT_PATTERN = /\b(?:eslint|oxlint|lint-staged|lint)\b/i;
    REMOTE_PUSH_EXCLUSION_PATTERN = /authentication failed|repository not found|not a git repository|does not appear to be a git repository|permission denied|protected branch|pre-receive hook declined|non-fast-forward|fetch first|updates were rejected|stale info|submodule|failed to push all needed submodules|unable to push submodule|unable to access|could not resolve host|network is unreachable|connection timed out|failed to connect|rpc failed|remote end hung up/i;
  }
});

// src/shared/git-remote-error.ts
function stripCredentialsFromMessage(message) {
  return message.replace(USERPASS_URL_PATTERN, "$1").replace(HTTPS_TOKEN_URL_PATTERN, "$1");
}
function formatSubmodulePushFailureDetail(message) {
  const raw = stripCredentialsFromMessage(message);
  const trimmed = raw.trim();
  const normalizedMatch = trimmed.match(NORMALIZED_SUBMODULE_PUSH_FAILURE_PATTERN);
  if (normalizedMatch) {
    return normalizedMatch[1];
  }
  if (!SUBMODULE_PUSH_FAILURE_SENTINEL_PATTERN.test(trimmed)) {
    return null;
  }
  const submoduleName = trimmed.match(SUBMODULE_PUSH_FAILURE_PATTERN)?.[1]?.trim();
  const subject = submoduleName ? `Submodule '${submoduleName}'` : "A submodule";
  if (SUBMODULE_REMOTE_CHANGED_PATTERN.test(trimmed)) {
    return `${subject} has remote changes. Pull inside the submodule, then try again.`;
  }
  return `${subject} could not be pushed. Resolve the submodule push error, then try again.`;
}
function extractTailLine(message) {
  for (const rawLine of iterateLinesFromEnd(message)) {
    const line = rawLine.trim();
    if (line.length > 0) {
      return line;
    }
  }
  return message;
}
function* iterateLinesFromEnd(value) {
  let lineEnd = value.length;
  let index = value.length - 1;
  while (index >= 0) {
    const code = value.charCodeAt(index);
    if (code !== 10 && code !== 13) {
      index--;
      continue;
    }
    const delimiterStart = code === 10 && index > 0 && value.charCodeAt(index - 1) === 13 ? index - 1 : index;
    yield value.slice(index + 1, lineEnd);
    lineEnd = delimiterStart;
    index = delimiterStart - 1;
  }
  yield value.slice(0, lineEnd);
}
function normalizeGitErrorMessage(error, operation) {
  if (!(error instanceof Error)) {
    return "Git remote operation failed.";
  }
  const raw = stripCredentialsFromMessage(error.message);
  const submodulePushFailureDetail = formatSubmodulePushFailureDetail(raw);
  if ((operation === "push" || operation === void 0) && submodulePushFailureDetail) {
    return submodulePushFailureDetail;
  }
  if ((operation === "push" || operation === void 0) && (raw.includes("non-fast-forward") || raw.includes("fetch first"))) {
    return "Push rejected: remote has newer commits (non-fast-forward). Please pull or sync first.";
  }
  if (operation === "push" && isPushHookFailure(raw)) {
    return raw.trim();
  }
  if (raw.includes("could not read Username") || raw.includes("Authentication failed")) {
    return "Authentication failed. Check your remote credentials.";
  }
  if (raw.includes("Could not resolve host") || raw.includes("Network is unreachable")) {
    return "Network error. Check your connection.";
  }
  if (raw.includes("no tracking information") || raw.includes("no upstream")) {
    return "Branch has no upstream. Publish the branch first.";
  }
  if (operation === "pull" && DIVERGENT_PULL_RECONCILIATION_PATTERN.test(raw)) {
    return "Pull needs a Git pull policy for divergent branches. Configure one for this repository or host, then try again: git config pull.rebase false (merge), git config pull.rebase true (rebase), or git config pull.ff only (fast-forward only).";
  }
  if (raw.includes("Your local changes to the following files would be overwritten") || raw.includes("Your local changes would be overwritten")) {
    return "Pull would overwrite local changes. Commit, stash, or discard them before pulling.";
  }
  if (raw.includes("untracked working tree files would be overwritten")) {
    return "Pull would overwrite untracked files. Move, remove, or add them before pulling.";
  }
  return extractTailLine(raw);
}
function isNoUpstreamError(error) {
  if (!(error instanceof Error)) {
    return false;
  }
  const message = error.message;
  return FATAL_PREFIX_PATTERN.test(message) && NO_UPSTREAM_PHRASE_PATTERN.test(message);
}
var USERPASS_URL_PATTERN, HTTPS_TOKEN_URL_PATTERN, SUBMODULE_PUSH_FAILURE_PATTERN, SUBMODULE_PUSH_FAILURE_SENTINEL_PATTERN, SUBMODULE_REMOTE_CHANGED_PATTERN, NORMALIZED_SUBMODULE_PUSH_FAILURE_PATTERN, DIVERGENT_PULL_RECONCILIATION_PATTERN, NO_UPSTREAM_PHRASE_PATTERN, FATAL_PREFIX_PATTERN;
var init_git_remote_error = __esm({
  "src/shared/git-remote-error.ts"() {
    "use strict";
    init_source_control_push_failure();
    USERPASS_URL_PATTERN = /([a-z][a-z0-9+.-]*:\/\/)[^\s/@:]+:[^\s/@]+@/gi;
    HTTPS_TOKEN_URL_PATTERN = /(https?:\/\/)[^\s/@:]+@/gi;
    SUBMODULE_PUSH_FAILURE_PATTERN = /Unable to push submodule ['"](.+?)['"]/i;
    SUBMODULE_PUSH_FAILURE_SENTINEL_PATTERN = /failed to push all needed submodules|Unable to push submodule/i;
    SUBMODULE_REMOTE_CHANGED_PATTERN = /non-fast-forward|fetch first|updates were rejected|remote contains work that you do not have/i;
    NORMALIZED_SUBMODULE_PUSH_FAILURE_PATTERN = /(?:^|:\s)((?:Submodule '[^'\n]+'|A submodule) (?:has remote changes\. Pull inside the submodule, then try again\.|could not be pushed\. Resolve the submodule push error, then try again\.))(?:$|\s)/i;
    DIVERGENT_PULL_RECONCILIATION_PATTERN = /Need to specify how to reconcile divergent branches|divergent branches and need to specify how to reconcile them/i;
    NO_UPSTREAM_PHRASE_PATTERN = /no upstream configured|no tracking information|HEAD does not point|Needed a single revision|ambiguous argument 'HEAD@\{u\}'/i;
    FATAL_PREFIX_PATTERN = /(^|\n)fatal:/i;
  }
});

// src/shared/git-remote-branch-name.ts
function splitRemoteBranchName(refName) {
  const slashIndex = refName.indexOf("/");
  if (slashIndex <= 0 || slashIndex === refName.length - 1) {
    return null;
  }
  return {
    remoteName: refName.slice(0, slashIndex),
    branchName: refName.slice(slashIndex + 1)
  };
}
function gitRefTargetsBranchOnRemote(refName, remoteName, branchName) {
  const trimmed = refName?.trim();
  if (!trimmed || !remoteName || !branchName) {
    return false;
  }
  if (trimmed === `${remoteName}/${branchName}` || trimmed === `remotes/${remoteName}/${branchName}` || trimmed === `refs/remotes/${remoteName}/${branchName}`) {
    return true;
  }
  if (trimmed.startsWith("refs/remotes/") || trimmed.startsWith("remotes/")) {
    return false;
  }
  const headsPrefix = "refs/heads/";
  if (trimmed.startsWith(headsPrefix)) {
    return trimmed.slice(headsPrefix.length) === branchName;
  }
  return trimmed === branchName;
}
var init_git_remote_branch_name = __esm({
  "src/shared/git-remote-branch-name.ts"() {
    "use strict";
  }
});

// src/shared/git-configured-branch-target.ts
async function getGitConfigValue(runGit, key) {
  try {
    const { stdout } = await runGit(["config", "--get", key]);
    const value = stdout.trim();
    return value || null;
  } catch {
    return null;
  }
}
function isUrlValuedRemote(remote) {
  return /^[A-Za-z][A-Za-z0-9+.-]*:\/\//.test(remote) || /^[^@/:]+@[^:]+:.+/.test(remote);
}
async function findRemoteNameForUrl(runGit, remoteUrl) {
  try {
    const { stdout } = await runGit(["remote"]);
    const remotes = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    for (const remoteName of remotes) {
      try {
        const { stdout: urlStdout } = await runGit(["remote", "get-url", remoteName]);
        if (urlStdout.trim() === remoteUrl) {
          return remoteName;
        }
      } catch {
      }
    }
  } catch {
    return null;
  }
  return null;
}
async function getConfiguredBranchRemoteUpstream(runGit, currentBranchName, remoteTrackingRefExists2) {
  const [remote, mergeRef, baseRef] = await Promise.all([
    getGitConfigValue(runGit, `branch.${currentBranchName}.remote`),
    getGitConfigValue(runGit, `branch.${currentBranchName}.merge`),
    getGitConfigValue(runGit, `branch.${currentBranchName}.base`)
  ]);
  const branchName = mergeRef?.replace(/^refs\/heads\//, "") ?? "";
  if (!remote || !branchName || branchName === mergeRef || remote === ".") {
    return null;
  }
  const remoteName = isUrlValuedRemote(remote) ? await findRemoteNameForUrl(runGit, remote) : remote;
  if (!remoteName || gitRefTargetsBranchOnRemote(baseRef, remoteName, branchName) || !await remoteTrackingRefExists2(remoteName, branchName)) {
    return null;
  }
  return {
    upstreamName: `${remoteName}/${branchName}`,
    remoteName,
    branchName,
    isConfiguredUpstream: false
  };
}
async function hasConfiguredBranchPushTarget(runGit, currentBranchName) {
  const [pushRemote, pushDefault, branchRemote, mergeRef, baseRef] = await Promise.all([
    getGitConfigValue(runGit, `branch.${currentBranchName}.pushRemote`),
    getGitConfigValue(runGit, "remote.pushDefault"),
    getGitConfigValue(runGit, `branch.${currentBranchName}.remote`),
    getGitConfigValue(runGit, `branch.${currentBranchName}.merge`),
    getGitConfigValue(runGit, `branch.${currentBranchName}.base`)
  ]);
  const remote = pushRemote ?? pushDefault ?? branchRemote;
  const branchName = mergeRef?.replace(/^refs\/heads\//, "") ?? "";
  if (!remote || remote === "." || !branchName || branchName === mergeRef) {
    return false;
  }
  const pushRemoteName = isUrlValuedRemote(remote) ? await findRemoteNameForUrl(runGit, remote) ?? remote : remote;
  const branchRemoteName = branchRemote ? isUrlValuedRemote(branchRemote) ? await findRemoteNameForUrl(runGit, branchRemote) ?? branchRemote : branchRemote : null;
  if (gitRefTargetsBranchOnRemote(baseRef, pushRemoteName, branchName)) {
    return false;
  }
  if (branchName !== currentBranchName && (pushRemoteName === "origin" || branchRemoteName !== pushRemoteName)) {
    return false;
  }
  return true;
}
var init_git_configured_branch_target = __esm({
  "src/shared/git-configured-branch-target.ts"() {
    "use strict";
    init_git_remote_branch_name();
  }
});

// src/shared/git-effective-upstream.ts
function hasMultipleSlashSegments(refName) {
  return refName.includes("/") && refName.indexOf("/") !== refName.lastIndexOf("/");
}
async function splitRemoteBranchNameByKnownRemote(runGit, refName) {
  try {
    const { stdout } = await runGit(["remote"]);
    let bestRemoteName = null;
    for (const rawLine of iterateProcessOutputLines(stdout)) {
      const remoteName = rawLine.trim();
      if (!remoteName || refName === remoteName || !refName.startsWith(`${remoteName}/`) || bestRemoteName && bestRemoteName.length >= remoteName.length) {
        continue;
      }
      bestRemoteName = remoteName;
    }
    if (!bestRemoteName) {
      return null;
    }
    const branchName = refName.slice(bestRemoteName.length + 1);
    return branchName ? { remoteName: bestRemoteName, branchName } : null;
  } catch {
    return null;
  }
}
async function getCurrentBranchName(runGit) {
  try {
    const { stdout } = await runGit(["symbolic-ref", "--quiet", "--short", "HEAD"]);
    const branchName = stdout.trim();
    return branchName || null;
  } catch {
    return null;
  }
}
async function getConfiguredUpstream(runGit) {
  try {
    const { stdout } = await runGit(["rev-parse", "--abbrev-ref", "HEAD@{u}"]);
    const upstreamName = stdout.trim();
    if (!upstreamName) {
      return null;
    }
    const parsed = splitRemoteBranchName(upstreamName);
    if (!parsed) {
      return {
        upstreamName,
        remoteName: null,
        branchName: upstreamName,
        isConfiguredUpstream: true
      };
    }
    return {
      upstreamName,
      remoteName: parsed.remoteName,
      branchName: parsed.branchName,
      isConfiguredUpstream: true
    };
  } catch (error) {
    if (isNoUpstreamError(error)) {
      return null;
    }
    throw error;
  }
}
async function remoteTrackingRefExists(runGit, remoteName, branchName) {
  try {
    await runGit(["rev-parse", "--verify", "--quiet", `refs/remotes/${remoteName}/${branchName}`]);
    return true;
  } catch {
    return false;
  }
}
async function resolveEffectiveGitUpstreamForBranch(runGit, currentBranchName) {
  let configured = await getConfiguredUpstream(runGit);
  if (configured) {
    if (currentBranchName && configured.remoteName === "origin" && configured.branchName !== currentBranchName && hasMultipleSlashSegments(configured.upstreamName)) {
      const parsed = await splitRemoteBranchNameByKnownRemote(runGit, configured.upstreamName);
      if (parsed) {
        configured = { ...configured, ...parsed };
      }
    }
    if (!currentBranchName || configured.branchName === currentBranchName) {
      return configured;
    }
    if (configured.remoteName === "origin" && await remoteTrackingRefExists(runGit, configured.remoteName, currentBranchName)) {
      return {
        upstreamName: `${configured.remoteName}/${currentBranchName}`,
        remoteName: configured.remoteName,
        branchName: currentBranchName,
        isConfiguredUpstream: false
      };
    }
    return configured;
  }
  if (currentBranchName) {
    const branchRemoteUpstream = await getConfiguredBranchRemoteUpstream(
      runGit,
      currentBranchName,
      (remoteName, branchName) => remoteTrackingRefExists(runGit, remoteName, branchName)
    );
    if (branchRemoteUpstream) {
      return branchRemoteUpstream;
    }
  }
  if (currentBranchName && await remoteTrackingRefExists(runGit, "origin", currentBranchName)) {
    return {
      upstreamName: `origin/${currentBranchName}`,
      remoteName: "origin",
      branchName: currentBranchName,
      isConfiguredUpstream: false
    };
  }
  return null;
}
async function resolveEffectiveGitUpstream(runGit) {
  return resolveEffectiveGitUpstreamForBranch(runGit, await getCurrentBranchName(runGit));
}
async function getEffectiveGitUpstreamStatus(runGit, getBehindCommitsArePatchEquivalent) {
  const currentBranchName = await getCurrentBranchName(runGit);
  const upstream = await resolveEffectiveGitUpstreamForBranch(runGit, currentBranchName);
  if (!upstream) {
    const hasConfiguredPushTarget = currentBranchName ? await hasConfiguredBranchPushTarget(runGit, currentBranchName) : false;
    return {
      hasUpstream: false,
      ahead: 0,
      behind: 0,
      ...hasConfiguredPushTarget ? { hasConfiguredPushTarget: true } : {}
    };
  }
  return getGitUpstreamStatusForUpstreamName(
    runGit,
    upstream.upstreamName,
    getBehindCommitsArePatchEquivalent
  );
}
async function getGitUpstreamStatusForUpstreamName(runGit, upstreamName, getBehindCommitsArePatchEquivalent) {
  const { stdout } = await runGit(["rev-list", "--left-right", "--count", `HEAD...${upstreamName}`]);
  const counts = parseGitRevListAheadBehindCounts(stdout);
  if (counts.status === "unexpected-field-count") {
    throw new Error(`Unexpected git rev-list output: ${JSON.stringify(stdout)}`);
  }
  if (counts.status === "unparseable-counts") {
    throw new Error(`Unparseable git rev-list counts: ${JSON.stringify(stdout)}`);
  }
  const behindCommitsArePatchEquivalent = counts.ahead > 0 && counts.behind > 0 && getBehindCommitsArePatchEquivalent ? await getBehindCommitsArePatchEquivalent(upstreamName) : void 0;
  return {
    hasUpstream: true,
    upstreamName,
    ahead: counts.ahead,
    behind: counts.behind,
    ...behindCommitsArePatchEquivalent !== void 0 ? { behindCommitsArePatchEquivalent } : {}
  };
}
var init_git_effective_upstream = __esm({
  "src/shared/git-effective-upstream.ts"() {
    "use strict";
    init_git_remote_error();
    init_git_configured_branch_target();
    init_git_remote_branch_name();
    init_git_rev_list_output();
    init_process_output_field_scanner();
    init_git_remote_branch_name();
  }
});

// src/shared/git-config-snapshot-runner.ts
function isConfigGetCommand(args) {
  return args.length === 3 && args[0] === "config" && args[1] === "--get";
}
function canonicalizeGitConfigLookupKey(key) {
  const parts = key.split(".");
  if (parts.length === 1) {
    return key.toLowerCase();
  }
  const firstPart = parts[0]?.toLowerCase() ?? "";
  const lastPart = parts.at(-1)?.toLowerCase() ?? "";
  return [firstPart, ...parts.slice(1, -1), lastPart].join(".");
}
function parseGitConfigListSnapshot(stdout) {
  const snapshot = /* @__PURE__ */ new Map();
  for (const record of stdout.split("\0")) {
    if (!record.trim()) {
      continue;
    }
    const separatorIndex = record.indexOf("\n");
    const key = separatorIndex === -1 ? record : record.slice(0, separatorIndex);
    const value = separatorIndex === -1 ? "" : record.slice(separatorIndex + 1);
    const values = snapshot.get(key) ?? [];
    values.push(value);
    snapshot.set(key, values);
  }
  return snapshot;
}
function createGitConfigSnapshotRunner(runGit) {
  let snapshotPromise = null;
  let snapshot = null;
  let interceptionDisabled = false;
  const readSnapshot = () => {
    if (snapshot) {
      return Promise.resolve(snapshot);
    }
    if (!snapshotPromise) {
      try {
        snapshotPromise = runGit(["config", "--list", "-z"]).then(({ stdout }) => {
          snapshot = parseGitConfigListSnapshot(stdout);
          return snapshot;
        }).catch(() => {
          interceptionDisabled = true;
          snapshotPromise = null;
          return null;
        });
      } catch {
        interceptionDisabled = true;
        snapshotPromise = null;
        return Promise.resolve(null);
      }
    }
    return snapshotPromise;
  };
  return async (args) => {
    if (interceptionDisabled || !isConfigGetCommand(args)) {
      return runGit(args);
    }
    const configSnapshot = await readSnapshot();
    if (!configSnapshot) {
      return runGit(args);
    }
    const key = args[2] ?? "";
    const values = configSnapshot.get(canonicalizeGitConfigLookupKey(key));
    if (!values?.length) {
      throw new GitConfigSnapshotKeyNotFoundError(key);
    }
    return { stdout: values.at(-1) ?? "" };
  };
}
var GitConfigSnapshotKeyNotFoundError;
var init_git_config_snapshot_runner = __esm({
  "src/shared/git-config-snapshot-runner.ts"() {
    "use strict";
    GitConfigSnapshotKeyNotFoundError = class extends Error {
      constructor(key) {
        super(`git config --get found no value for '${key}'`);
        this.name = "GitConfigSnapshotKeyNotFoundError";
      }
    };
  }
});

// src/relay/git-status-upstream-negative-cache.ts
function noEffectiveUpstreamCacheKey(identity) {
  return [identity.worktreePath, identity.branchName, identity.upstreamName ?? ""].join("\0");
}
function readCachedNoEffectiveUpstreamStatus(cacheKey, nowMs = Date.now()) {
  const entry = noEffectiveUpstreamByIdentity.get(cacheKey);
  if (!entry) {
    return null;
  }
  if (entry.expiresAt <= nowMs) {
    noEffectiveUpstreamByIdentity.delete(cacheKey);
    return null;
  }
  return entry.status;
}
function hasPendingNoEffectiveUpstreamProbe(cacheKey) {
  return noEffectiveUpstreamInFlight.has(cacheKey) || retiredNoEffectiveUpstreamInFlight.has(cacheKey);
}
function trimNoEffectiveUpstreamWriteGeneration() {
  for (const cacheKey of noEffectiveUpstreamWriteGeneration.keys()) {
    if (noEffectiveUpstreamWriteGeneration.size <= MAX_NO_EFFECTIVE_UPSTREAM_CACHE_ENTRIES) {
      break;
    }
    if (hasPendingNoEffectiveUpstreamProbe(cacheKey)) {
      continue;
    }
    noEffectiveUpstreamWriteGeneration.delete(cacheKey);
  }
}
function cacheNoEffectiveUpstreamStatus(cacheKey, status, probedSameNameOriginRef, writeGeneration, nowMs = Date.now()) {
  if (status.hasUpstream || status.hasConfiguredPushTarget) {
    noEffectiveUpstreamByIdentity.delete(cacheKey);
    noEffectiveUpstreamWriteGeneration.set(cacheKey, writeGeneration + 1);
    trimNoEffectiveUpstreamWriteGeneration();
    return;
  }
  if ((noEffectiveUpstreamWriteGeneration.get(cacheKey) ?? 0) !== writeGeneration) {
    return;
  }
  if (!probedSameNameOriginRef) {
    return;
  }
  noEffectiveUpstreamByIdentity.set(cacheKey, {
    status,
    expiresAt: nowMs + NO_EFFECTIVE_UPSTREAM_CACHE_TTL_MS
  });
  while (noEffectiveUpstreamByIdentity.size > MAX_NO_EFFECTIVE_UPSTREAM_CACHE_ENTRIES) {
    const oldest = noEffectiveUpstreamByIdentity.keys().next();
    if (oldest.done) {
      break;
    }
    noEffectiveUpstreamByIdentity.delete(oldest.value);
    noEffectiveUpstreamWriteGeneration.delete(oldest.value);
  }
  trimNoEffectiveUpstreamWriteGeneration();
}
async function readOrProbeNoEffectiveUpstreamStatus(identity, runGit, options = {}) {
  const cacheKey = noEffectiveUpstreamCacheKey(identity);
  if (options.bypassCache !== true) {
    const cachedStatus = readCachedNoEffectiveUpstreamStatus(cacheKey);
    if (cachedStatus) {
      return cachedStatus;
    }
    const inFlight = noEffectiveUpstreamInFlight.get(cacheKey);
    if (inFlight) {
      return inFlight;
    }
  }
  let probedSameNameOriginRef = false;
  const snapshotRunner = createGitConfigSnapshotRunner(runGit);
  const writeGeneration = noEffectiveUpstreamWriteGeneration.get(cacheKey) ?? 0;
  const probe = getEffectiveGitUpstreamStatus((args) => {
    if (args[0] === "rev-parse" && args.includes(`refs/remotes/origin/${identity.branchName}`)) {
      probedSameNameOriginRef = true;
    }
    return snapshotRunner(args);
  }).then((status) => {
    cacheNoEffectiveUpstreamStatus(cacheKey, status, probedSameNameOriginRef, writeGeneration);
    return status;
  });
  if (options.bypassCache !== true) {
    noEffectiveUpstreamInFlight.set(cacheKey, probe);
  }
  try {
    return await probe;
  } finally {
    if (noEffectiveUpstreamInFlight.get(cacheKey) === probe) {
      noEffectiveUpstreamInFlight.delete(cacheKey);
      trimNoEffectiveUpstreamWriteGeneration();
    }
  }
}
var NO_EFFECTIVE_UPSTREAM_CACHE_TTL_MS, MAX_NO_EFFECTIVE_UPSTREAM_CACHE_ENTRIES, noEffectiveUpstreamByIdentity, noEffectiveUpstreamInFlight, retiredNoEffectiveUpstreamInFlight, noEffectiveUpstreamWriteGeneration;
var init_git_status_upstream_negative_cache = __esm({
  "src/relay/git-status-upstream-negative-cache.ts"() {
    "use strict";
    init_git_config_snapshot_runner();
    init_git_effective_upstream();
    NO_EFFECTIVE_UPSTREAM_CACHE_TTL_MS = 5 * 6e4;
    MAX_NO_EFFECTIVE_UPSTREAM_CACHE_ENTRIES = 512;
    noEffectiveUpstreamByIdentity = /* @__PURE__ */ new Map();
    noEffectiveUpstreamInFlight = /* @__PURE__ */ new Map();
    retiredNoEffectiveUpstreamInFlight = /* @__PURE__ */ new Map();
    noEffectiveUpstreamWriteGeneration = /* @__PURE__ */ new Map();
  }
});

// src/relay/git-handler-status-ops.ts
async function resolveGitDir(worktreePath) {
  const dotGitPath = path8.join(worktreePath, ".git");
  try {
    const contents = await (0, import_promises4.readFile)(dotGitPath, "utf-8");
    const match = contents.match(/^gitdir:\s*(.+)\s*$/m);
    if (match) {
      return path8.resolve(worktreePath, match[1]);
    }
  } catch {
  }
  return dotGitPath;
}
async function detectConflictOperation(worktreePath) {
  const gitDir = await resolveGitDir(worktreePath);
  try {
    if ((0, import_node_fs3.existsSync)(path8.join(gitDir, "MERGE_HEAD"))) {
      return "merge";
    }
    if ((0, import_node_fs3.existsSync)(path8.join(gitDir, "rebase-merge")) || (0, import_node_fs3.existsSync)(path8.join(gitDir, "rebase-apply"))) {
      return "rebase";
    }
    if ((0, import_node_fs3.existsSync)(path8.join(gitDir, "CHERRY_PICK_HEAD"))) {
      return "cherry-pick";
    }
  } catch {
  }
  return "unknown";
}
async function getStatusOp(git, params) {
  const worktreePath = params.worktreePath;
  const includeIgnored = params.includeIgnored === true;
  const rawLimit = params.limit;
  const limit = typeof rawLimit === "number" && Number.isFinite(rawLimit) && rawLimit >= 0 ? Math.floor(rawLimit) : DEFAULT_GIT_STATUS_LIMIT;
  const conflictOperation = await detectConflictOperation(worktreePath);
  const entries = [];
  let head;
  let branch;
  let upstreamStatus;
  let ignoredPaths = [];
  let didHitLimit = false;
  let statusLength = 0;
  try {
    const statusArgs = [
      "-c",
      "core.quotePath=false",
      "status",
      "--porcelain=v2",
      "--branch",
      "--untracked-files=all"
    ];
    if (includeIgnored) {
      statusArgs.push("--ignored=matching");
    }
    const { stdout } = await git(statusArgs, worktreePath, {
      // Why: status polling is read-like; avoid refreshing the index and racing
      // terminal Git commands on `.git/worktrees/*/index.lock`.
      disableOptionalLocks: true
    });
    const parsed = parseStatusOutput(stdout);
    head = parsed.head;
    branch = parsed.branch;
    upstreamStatus = parsed.upstreamStatus;
    ignoredPaths = parsed.ignoredPaths;
    statusLength = parsed.entries.length;
    if (limit !== 0 && parsed.entries.length > limit) {
      didHitLimit = true;
      for (let i = 0; i < limit; i++) {
        entries.push(parsed.entries[i]);
      }
    } else {
      for (const entry of parsed.entries) {
        entries.push(entry);
      }
    }
    if (!didHitLimit) {
      if (shouldProbeEffectiveUpstreamStatus(branch, upstreamStatus?.upstreamName)) {
        const branchName = getShortBranchName(branch);
        if (branchName) {
          try {
            upstreamStatus = await readOrProbeNoEffectiveUpstreamStatus(
              { worktreePath, branchName, upstreamName: upstreamStatus?.upstreamName },
              (args) => git(args, worktreePath),
              {
                bypassCache: params.bypassEffectiveUpstreamNegativeCache === true
              }
            );
          } catch {
          }
        }
      }
      for (const uLine of parsed.unmergedLines) {
        const entry = parseUnmergedEntry(worktreePath, uLine);
        if (entry) {
          entries.push(entry);
        }
      }
    }
  } catch {
  }
  if (!didHitLimit) {
    await attachLineStats(git, worktreePath, entries);
  }
  return {
    entries,
    conflictOperation,
    head,
    branch,
    upstreamStatus,
    ...includeIgnored ? { ignoredPaths } : {},
    ...didHitLimit ? { didHitLimit: true, statusLength } : {}
  };
}
async function runNumstat(git, worktreePath, cached) {
  try {
    const { stdout } = await git(
      ["-c", "core.quotePath=false", "diff", ...cached ? ["--cached"] : [], "--numstat", "-M"],
      worktreePath,
      { disableOptionalLocks: true }
    );
    return parseNumstat(stdout);
  } catch {
    return /* @__PURE__ */ new Map();
  }
}
async function attachLineStats(git, worktreePath, entries) {
  if (entries.length === 0) {
    return;
  }
  const hasStaged = entries.some((entry) => entry.area === "staged");
  const hasUnstaged = entries.some((entry) => entry.area === "unstaged");
  const untrackedPaths = entries.filter((entry) => entry.area === "untracked").map((entry) => entry.path);
  const emptyStats = /* @__PURE__ */ new Map();
  const [stagedStats, unstagedStats, untrackedStats] = await Promise.all([
    hasStaged ? runNumstat(git, worktreePath, true) : Promise.resolve(emptyStats),
    hasUnstaged ? runNumstat(git, worktreePath, false) : Promise.resolve(emptyStats),
    collectUntrackedAdditions(worktreePath, untrackedPaths)
  ]);
  for (const entry of entries) {
    const filePath = entry.path;
    applyLineStats(
      entry,
      entry.area === "staged" ? stagedStats.get(filePath) : entry.area === "unstaged" ? unstagedStats.get(filePath) : untrackedStats.get(filePath)
    );
  }
}
function getShortBranchName(branch) {
  const prefix = "refs/heads/";
  return branch?.startsWith(prefix) ? branch.slice(prefix.length) : null;
}
function shouldProbeEffectiveUpstreamStatus(branch, upstreamName) {
  const branchName = getShortBranchName(branch);
  if (!branchName) {
    return false;
  }
  if (!upstreamName) {
    return true;
  }
  const parsed = splitRemoteBranchName(upstreamName);
  return parsed?.remoteName === "origin" && parsed.branchName !== branchName;
}
var path8, import_node_fs3, import_promises4;
var init_git_handler_status_ops = __esm({
  "src/relay/git-handler-status-ops.ts"() {
    "use strict";
    path8 = __toESM(require("node:path"));
    import_node_fs3 = require("node:fs");
    import_promises4 = require("node:fs/promises");
    init_git_handler_utils();
    init_git_status_output_parser();
    init_git_effective_upstream();
    init_git_status_upstream_negative_cache();
    init_git_uncommitted_line_stats();
    init_git_status_limit();
  }
});

// src/shared/git-check-ignore-stdio.ts
function encodeGitCheckIgnorePaths(paths) {
  return `${paths.join("\0")}\0`;
}
function splitGitCheckIgnorePathsByStdinBytes(paths, maxBytes = GIT_CHECK_IGNORE_STDIN_CHUNK_BYTES) {
  const encoder = new TextEncoder();
  const chunks = [];
  let chunk = [];
  let chunkBytes = 0;
  for (const path12 of paths) {
    const pathBytes = encoder.encode(path12).byteLength + 1;
    if (chunk.length > 0 && chunkBytes + pathBytes > maxBytes) {
      chunks.push(chunk);
      chunk = [];
      chunkBytes = 0;
    }
    chunk.push(path12);
    chunkBytes += pathBytes;
  }
  if (chunk.length > 0) {
    chunks.push(chunk);
  }
  return chunks;
}
function parseGitCheckIgnorePaths(stdout) {
  return stdout.split("\0").filter((path12) => path12.length > 0);
}
var GIT_CHECK_IGNORE_TIMEOUT_MS, GIT_CHECK_IGNORE_STDIN_CHUNK_BYTES, GIT_CHECK_IGNORE_STDIN_ARGS;
var init_git_check_ignore_stdio = __esm({
  "src/shared/git-check-ignore-stdio.ts"() {
    "use strict";
    GIT_CHECK_IGNORE_TIMEOUT_MS = 15e3;
    GIT_CHECK_IGNORE_STDIN_CHUNK_BYTES = 1024 * 1024;
    GIT_CHECK_IGNORE_STDIN_ARGS = [
      "-c",
      "core.quotePath=false",
      "check-ignore",
      "-z",
      "--stdin"
    ];
  }
});

// src/relay/git-handler-check-ignore.ts
async function checkIgnoredPathsOp(git, params) {
  const worktreePath = params.worktreePath;
  const paths = Array.isArray(params.paths) ? params.paths.filter((path12) => typeof path12 === "string" && path12.length > 0) : [];
  const ignored = [];
  for (const chunk of splitGitCheckIgnorePathsByStdinBytes(paths)) {
    try {
      const { stdout } = await git([...GIT_CHECK_IGNORE_STDIN_ARGS], worktreePath, {
        stdin: encodeGitCheckIgnorePaths(chunk),
        timeout: GIT_CHECK_IGNORE_TIMEOUT_MS
      });
      ignored.push(...parseGitCheckIgnorePaths(stdout));
    } catch (error) {
      const gitError = error;
      if (gitError.code !== 1) {
        throw error;
      }
      ignored.push(...parseGitCheckIgnorePaths(gitError.stdout ?? ""));
    }
  }
  return ignored;
}
var init_git_handler_check_ignore = __esm({
  "src/relay/git-handler-check-ignore.ts"() {
    "use strict";
    init_git_check_ignore_stdio();
  }
});

// src/shared/git-push-target-validation.ts
function assertString(value, name) {
  if (typeof value !== "string") {
    throw new Error(`Invalid PR push target ${name}.`);
  }
}
function isSafeRemoteName(remoteName) {
  if (remoteName.length === 0 || remoteName.length > 100) {
    return false;
  }
  return remoteName.split("/").every((segment) => {
    return segment !== "" && segment !== "." && segment !== ".." && SAFE_REMOTE_NAME_SEGMENT.test(segment);
  });
}
function assertGitPushTargetShape(target) {
  if (typeof target !== "object" || target === null) {
    throw new Error("Invalid PR push target.");
  }
  const candidate = target;
  assertString(candidate.remoteName, "remote name");
  assertString(candidate.branchName, "branch name");
  if (!isSafeRemoteName(candidate.remoteName)) {
    throw new Error(`Invalid git remote name: ${candidate.remoteName}`);
  }
  if (!candidate.branchName || candidate.branchName.startsWith("-")) {
    throw new Error(`Invalid git branch name: ${candidate.branchName}`);
  }
  if (candidate.remoteUrl !== void 0) {
    assertString(candidate.remoteUrl, "remote URL");
    if (!(GITHUB_CLONE_URL.test(candidate.remoteUrl) || GITHUB_SSH_URL.test(candidate.remoteUrl))) {
      throw new Error("Invalid PR push target remote URL.");
    }
  }
}
var SAFE_REMOTE_NAME_SEGMENT, GITHUB_CLONE_URL, GITHUB_SSH_URL;
var init_git_push_target_validation = __esm({
  "src/shared/git-push-target-validation.ts"() {
    "use strict";
    SAFE_REMOTE_NAME_SEGMENT = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
    GITHUB_CLONE_URL = /^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\.git$/;
    GITHUB_SSH_URL = /^git@github\.com:[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\.git$/;
  }
});

// src/relay/git-handler-push-target.ts
async function getConfiguredPushTarget(git, worktreePath) {
  try {
    const { stdout: branchStdout } = await git(
      ["symbolic-ref", "--quiet", "--short", "HEAD"],
      worktreePath
    );
    const branch = branchStdout.trim();
    if (!branch) {
      return null;
    }
    const [pushRemote, { stdout: mergeStdout }] = await Promise.all([
      getConfiguredPushRemote(git, worktreePath, branch),
      git(["config", "--get", `branch.${branch}.merge`], worktreePath)
    ]);
    const remote = pushRemote?.remote;
    const mergeRef = mergeStdout.trim();
    const branchRef = mergeRef.replace(/^refs\/heads\//, "");
    if (!remote || !branchRef || remote === "." || branchRef === mergeRef) {
      return null;
    }
    if (await branchMergeTargetsConfiguredBase(git, worktreePath, branch, remote, branchRef)) {
      return null;
    }
    if (!canPushConfiguredMergeBranch(pushRemote, branch, branchRef)) {
      return null;
    }
    return { remote, refspec: `HEAD:${branchRef}` };
  } catch {
    return null;
  }
}
async function getConfigValue(git, worktreePath, key) {
  try {
    const { stdout } = await git(["config", "--get", key], worktreePath);
    const value = stdout.trim();
    return value || null;
  } catch {
    return null;
  }
}
function isUrlValuedRemote2(remote) {
  return /^[A-Za-z][A-Za-z0-9+.-]*:\/\//.test(remote) || /^[^@/:]+@[^:]+:.+/.test(remote);
}
async function findRemoteNameForUrl2(git, worktreePath, remoteUrl) {
  try {
    const { stdout } = await git(["remote"], worktreePath);
    const remotes = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    for (const remoteName of remotes) {
      try {
        const { stdout: urlStdout } = await git(["remote", "get-url", remoteName], worktreePath);
        if (urlStdout.trim() === remoteUrl) {
          return remoteName;
        }
      } catch {
      }
    }
  } catch {
    return null;
  }
  return null;
}
async function normalizePushRemote(git, worktreePath, remote) {
  if (!isUrlValuedRemote2(remote)) {
    return remote;
  }
  return await findRemoteNameForUrl2(git, worktreePath, remote) ?? remote;
}
async function getConfiguredPushRemote(git, worktreePath, branch) {
  const branchRemote = await getConfigValue(git, worktreePath, `branch.${branch}.remote`);
  const remote = await getConfigValue(git, worktreePath, `branch.${branch}.pushRemote`) ?? await getConfigValue(git, worktreePath, "remote.pushDefault") ?? branchRemote;
  if (!remote) {
    return null;
  }
  return {
    remote: await normalizePushRemote(git, worktreePath, remote),
    branchRemote: branchRemote ? await normalizePushRemote(git, worktreePath, branchRemote) : null
  };
}
async function branchMergeTargetsConfiguredBase(git, worktreePath, branch, remote, branchRef) {
  return gitRefTargetsBranchOnRemote(
    await getConfigValue(git, worktreePath, `branch.${branch}.base`),
    remote,
    branchRef
  );
}
function canPushConfiguredMergeBranch(pushRemote, branch, branchRef) {
  if (!pushRemote) {
    return false;
  }
  if (branchRef === branch) {
    return true;
  }
  return pushRemote.remote !== "origin" && pushRemote.branchRemote === pushRemote.remote;
}
async function resolveRelayPushTarget(git, worktreePath, pushTarget) {
  if (pushTarget === void 0) {
    return getConfiguredPushTarget(git, worktreePath);
  }
  assertGitPushTargetShape(pushTarget);
  const explicitTarget = pushTarget;
  await git(["check-ref-format", "--branch", explicitTarget.branchName], worktreePath);
  return {
    remote: explicitTarget.remoteName,
    refspec: `HEAD:${explicitTarget.branchName}`
  };
}
var init_git_handler_push_target = __esm({
  "src/relay/git-handler-push-target.ts"() {
    "use strict";
    init_git_push_target_validation();
    init_git_remote_branch_name();
  }
});

// src/shared/git-upstream-status.ts
function upstreamOnlyCommitsArePatchEquivalent(cherryMarkOutput) {
  let hasCommit = false;
  for (const rawLine of iterateGitOutputLines(cherryMarkOutput)) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    hasCommit = true;
    if (!line.startsWith("=")) {
      return false;
    }
  }
  return hasCommit;
}
function* iterateGitOutputLines(output) {
  let lineStart = 0;
  for (let index = 0; index < output.length; index++) {
    const code = output.charCodeAt(index);
    if (code !== 10 && code !== 13) {
      continue;
    }
    yield output.slice(lineStart, index);
    if (code === 13 && output.charCodeAt(index + 1) === 10) {
      index++;
    }
    lineStart = index + 1;
  }
  if (lineStart <= output.length) {
    yield output.slice(lineStart);
  }
}
var init_git_upstream_status = __esm({
  "src/shared/git-upstream-status.ts"() {
    "use strict";
  }
});

// src/shared/git-publish-target-status.ts
function getPublishTargetDisplayName(target) {
  return `${target.remoteName}/${target.branchName}`;
}
function getPublishTargetRemoteRef(target) {
  return `refs/remotes/${target.remoteName}/${target.branchName}`;
}
function isMissingRemoteTrackingRefError(error) {
  if (!(error instanceof Error)) {
    return false;
  }
  const candidate = error;
  const stderr = typeof candidate.stderr === "string" ? candidate.stderr.trim() : "";
  if (stderr.length > 0) {
    return false;
  }
  return candidate.code === 1 || /(?:exited with|exit code) 1\b/i.test(candidate.message);
}
async function getPublishTargetStatus(runGit, target, getBehindCommitsArePatchEquivalent) {
  const upstreamName = getPublishTargetDisplayName(target);
  const remoteRef = getPublishTargetRemoteRef(target);
  try {
    await runGit(["rev-parse", "--verify", "--quiet", remoteRef]);
  } catch (error) {
    if (!isMissingRemoteTrackingRefError(error)) {
      throw error;
    }
    return {
      hasUpstream: false,
      upstreamName,
      ahead: 0,
      behind: 0,
      hasConfiguredPushTarget: true
    };
  }
  const { stdout } = await runGit(["rev-list", "--left-right", "--count", `HEAD...${remoteRef}`]);
  const counts = parseGitRevListAheadBehindCounts(stdout);
  if (counts.status === "unexpected-field-count") {
    throw new Error(`Unexpected git rev-list output: ${JSON.stringify(stdout)}`);
  }
  if (counts.status === "unparseable-counts") {
    throw new Error(`Unparseable git rev-list counts: ${JSON.stringify(stdout)}`);
  }
  const behindCommitsArePatchEquivalent = counts.ahead > 0 && counts.behind > 0 && getBehindCommitsArePatchEquivalent ? await getBehindCommitsArePatchEquivalent(remoteRef) : void 0;
  return {
    hasUpstream: true,
    upstreamName,
    ahead: counts.ahead,
    behind: counts.behind,
    ...behindCommitsArePatchEquivalent !== void 0 ? { behindCommitsArePatchEquivalent } : {}
  };
}
var init_git_publish_target_status = __esm({
  "src/shared/git-publish-target-status.ts"() {
    "use strict";
    init_git_rev_list_output();
  }
});

// src/shared/git-rebase-source.ts
function normalizeBaseRef(baseRef) {
  const trimmed = baseRef.trim();
  if (!trimmed || trimmed.startsWith("-")) {
    throw new Error("Choose a remote base branch to rebase from.");
  }
  if (trimmed.startsWith("refs/remotes/")) {
    return trimmed.slice("refs/remotes/".length);
  }
  if (trimmed.startsWith("remotes/")) {
    return trimmed.slice("remotes/".length);
  }
  return trimmed;
}
async function resolveGitRemoteRebaseSource(runGit, baseRef) {
  const normalizedBaseRef = normalizeBaseRef(baseRef);
  const { stdout } = await runGit(["remote"]);
  const remotes = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean).sort((a, b) => b.length - a.length);
  const remoteName = remotes.find(
    (remote) => normalizedBaseRef !== remote && normalizedBaseRef.startsWith(`${remote}/`)
  );
  if (!remoteName) {
    throw new Error("Choose a remote base branch to rebase from.");
  }
  const branchName = normalizedBaseRef.slice(remoteName.length + 1);
  await runGit(["check-ref-format", "--branch", branchName]);
  return {
    remoteName,
    branchName,
    displayName: `${remoteName}/${branchName}`
  };
}
var init_git_rebase_source = __esm({
  "src/shared/git-rebase-source.ts"() {
    "use strict";
  }
});

// src/shared/git-history-log-parser.ts
function shortGitHash(hash) {
  return hash.slice(0, 7);
}
function commitSubject(message) {
  const firstLine = message.split(/\r?\n/, 1)[0]?.trim();
  return firstLine || "(no commit message)";
}
function parseGitDecorationRefs(raw, revision) {
  if (!raw.trim()) {
    return [];
  }
  const refs = [];
  const parts = raw.includes(GIT_HISTORY_DECORATION_SEPARATOR) ? raw.split(GIT_HISTORY_DECORATION_SEPARATOR) : raw.split(",");
  for (const part of parts) {
    const ref = part.trim();
    if (!ref || ref === "HEAD" || /^refs\/remotes\/[^/]+\/HEAD(?:\s|$)/.test(ref)) {
      continue;
    }
    if (ref.startsWith("HEAD -> refs/heads/")) {
      refs.push({
        id: ref.slice("HEAD -> ".length),
        name: ref.slice("HEAD -> refs/heads/".length),
        revision,
        category: "branches"
      });
      continue;
    }
    if (ref.startsWith("refs/heads/")) {
      refs.push({
        id: ref,
        name: ref.slice("refs/heads/".length),
        revision,
        category: "branches"
      });
      continue;
    }
    if (ref.startsWith("refs/remotes/")) {
      refs.push({
        id: ref,
        name: ref.slice("refs/remotes/".length),
        revision,
        category: "remote branches"
      });
      continue;
    }
    if (ref.startsWith("tag: refs/tags/")) {
      refs.push({
        id: ref.slice("tag: ".length),
        name: ref.slice("tag: refs/tags/".length),
        revision,
        category: "tags"
      });
    }
  }
  return refs.sort(compareGitHistoryItemRefsByCategory);
}
function compareGitHistoryItemRefsByCategory(ref1, ref2) {
  const order = (ref) => {
    if (ref.id.startsWith("refs/heads/")) {
      return 1;
    }
    if (ref.id.startsWith("refs/remotes/")) {
      return 2;
    }
    if (ref.id.startsWith("refs/tags/")) {
      return 3;
    }
    return 99;
  };
  const categoryOrder = order(ref1) - order(ref2);
  return categoryOrder || ref1.name.localeCompare(ref2.name);
}
function parseGitHistoryLog(stdout) {
  const items = [];
  for (const rawRecord of stdout.split("\0")) {
    const record = rawRecord.replace(/^\n+/, "");
    if (!record.trim()) {
      continue;
    }
    const lines = record.split("\n");
    const hash = lines[0]?.trim() ?? "";
    if (!/^[0-9a-fA-F]{40,64}$/.test(hash)) {
      continue;
    }
    const authorName = lines[1] ?? "";
    const authorEmail = lines[2] ?? "";
    const authorDateSeconds = Number.parseInt(lines[3] ?? "", 10);
    const parents = (lines[5] ?? "").trim();
    const decorations = lines[6] ?? "";
    const message = lines.slice(7).join("\n").replace(/\n$/, "");
    items.push({
      id: hash,
      parentIds: parents ? parents.split(" ") : [],
      subject: commitSubject(message),
      message,
      author: authorName || void 0,
      authorEmail: authorEmail || void 0,
      displayId: shortGitHash(hash),
      timestamp: Number.isFinite(authorDateSeconds) ? authorDateSeconds * 1e3 : void 0,
      references: parseGitDecorationRefs(decorations, hash)
    });
  }
  return items;
}
function gitHistoryRefFromFullName(fullName, fallbackName, revision) {
  const id = fullName || fallbackName;
  if (id.startsWith("refs/heads/")) {
    return { id, name: id.slice("refs/heads/".length), revision, category: "branches" };
  }
  if (id.startsWith("refs/remotes/")) {
    return { id, name: id.slice("refs/remotes/".length), revision, category: "remote branches" };
  }
  if (id.startsWith("refs/tags/")) {
    return { id, name: id.slice("refs/tags/".length), revision, category: "tags" };
  }
  return { id, name: fallbackName || shortGitHash(revision), revision, category: "commits" };
}
var GIT_HISTORY_DECORATION_SEPARATOR, GIT_HISTORY_COMMIT_FORMAT;
var init_git_history_log_parser = __esm({
  "src/shared/git-history-log-parser.ts"() {
    "use strict";
    GIT_HISTORY_DECORATION_SEPARATOR = "";
    GIT_HISTORY_COMMIT_FORMAT = "%H%n%aN%n%aE%n%at%n%ct%n%P%n%(decorate:prefix=,suffix=,separator=%x1f)%n%B";
  }
});

// src/shared/git-history-types.ts
var GIT_HISTORY_DEFAULT_LIMIT, GIT_HISTORY_MAX_LIMIT;
var init_git_history_types = __esm({
  "src/shared/git-history-types.ts"() {
    "use strict";
    GIT_HISTORY_DEFAULT_LIMIT = 50;
    GIT_HISTORY_MAX_LIMIT = 200;
  }
});

// src/shared/git-history.ts
function clampHistoryLimit(limit) {
  if (!Number.isFinite(limit)) {
    return GIT_HISTORY_DEFAULT_LIMIT;
  }
  return Math.min(
    GIT_HISTORY_MAX_LIMIT,
    Math.max(1, Math.trunc(limit ?? GIT_HISTORY_DEFAULT_LIMIT))
  );
}
async function resolveCommit(git, cwd, ref) {
  if (!ref || ref.startsWith("-")) {
    return null;
  }
  try {
    const { stdout } = await git(
      ["rev-parse", "--verify", "--end-of-options", `${ref}^{commit}`],
      cwd
    );
    const oid = stdout.trim();
    return oid || null;
  } catch {
    return null;
  }
}
async function resolveSymbolicFullName(git, cwd, ref) {
  if (!ref || ref.startsWith("-")) {
    return null;
  }
  try {
    const { stdout } = await git(
      ["rev-parse", "--symbolic-full-name", "--end-of-options", ref],
      cwd
    );
    return stdout.trim().split(/\r?\n/).find(Boolean) ?? null;
  } catch {
    return null;
  }
}
async function resolveCurrentRef(git, cwd, headOid) {
  try {
    const { stdout } = await git(["symbolic-ref", "--quiet", "--short", "HEAD"], cwd);
    const branchName = stdout.trim();
    if (branchName) {
      return {
        branchName,
        currentRef: {
          id: `refs/heads/${branchName}`,
          name: branchName,
          revision: headOid,
          category: "branches"
        }
      };
    }
  } catch {
  }
  return {
    branchName: null,
    currentRef: { id: headOid, name: shortGitHash(headOid), revision: headOid, category: "commits" }
  };
}
async function resolveUpstreamRef(git, cwd, branchName) {
  if (!branchName) {
    return void 0;
  }
  try {
    const { stdout } = await git(
      ["for-each-ref", "--format=%(upstream)%00%(upstream:short)", `refs/heads/${branchName}`],
      cwd
    );
    const [fullName, shortName] = stdout.split("\0");
    const upstreamRef = fullName?.trim();
    const upstreamShortName = shortName?.trim();
    if (!upstreamRef || !upstreamShortName) {
      return void 0;
    }
    const oid = await resolveCommit(git, cwd, upstreamRef);
    return oid ? gitHistoryRefFromFullName(upstreamRef, upstreamShortName, oid) : void 0;
  } catch {
    return void 0;
  }
}
async function resolveNamedRef(git, cwd, ref) {
  const normalized = ref?.trim();
  if (!normalized || normalized.startsWith("-")) {
    return void 0;
  }
  const [revision, fullName] = await Promise.all([
    resolveCommit(git, cwd, normalized),
    resolveSymbolicFullName(git, cwd, normalized)
  ]);
  return revision ? gitHistoryRefFromFullName(fullName, normalized, revision) : void 0;
}
async function loadGitHistoryFromExecutor(git, cwd, options = {}) {
  const limit = clampHistoryLimit(options.limit);
  const headOid = await resolveCommit(git, cwd, "HEAD");
  if (!headOid) {
    return {
      items: [],
      hasIncomingChanges: false,
      hasOutgoingChanges: false,
      hasMore: false,
      limit
    };
  }
  const { currentRef, branchName } = await resolveCurrentRef(git, cwd, headOid);
  const [remoteRef, rawBaseRef] = await Promise.all([
    resolveUpstreamRef(git, cwd, branchName),
    resolveNamedRef(git, cwd, options.baseRef)
  ]);
  const baseRef = rawBaseRef && rawBaseRef.id !== remoteRef?.id && rawBaseRef.id !== currentRef.id ? rawBaseRef : void 0;
  const historyRevisions = [headOid];
  let mergeBase;
  if (remoteRef?.revision && currentRef.revision && remoteRef.revision !== currentRef.revision) {
    try {
      const { stdout: stdout2 } = await git(["merge-base", currentRef.revision, remoteRef.revision], cwd);
      mergeBase = stdout2.trim() || void 0;
    } catch {
      mergeBase = void 0;
    }
  }
  const { stdout } = await git(
    [
      "log",
      `--format=${GIT_HISTORY_COMMIT_FORMAT}`,
      "-z",
      "--topo-order",
      "--decorate=full",
      `-n${limit + 1}`,
      ...historyRevisions
    ],
    cwd
  );
  const parsed = parseGitHistoryLog(stdout);
  const items = parsed.slice(0, limit);
  const hasIncomingChanges = Boolean(remoteRef?.revision && mergeBase) && remoteRef?.revision !== mergeBase;
  const hasOutgoingChanges = Boolean(currentRef.revision && remoteRef?.revision && mergeBase) && currentRef.revision !== mergeBase;
  return {
    items,
    currentRef,
    remoteRef,
    baseRef,
    mergeBase,
    hasIncomingChanges,
    hasOutgoingChanges,
    hasMore: parsed.length > limit,
    limit
  };
}
var init_git_history = __esm({
  "src/shared/git-history.ts"() {
    "use strict";
    init_git_history_log_parser();
    init_git_history_types();
    init_git_history_types();
    init_git_history_log_parser();
  }
});

// src/shared/git-output-locale.ts
var UNTRANSLATED_GIT_OUTPUT_ENV;
var init_git_output_locale = __esm({
  "src/shared/git-output-locale.ts"() {
    "use strict";
    UNTRANSLATED_GIT_OUTPUT_ENV = {
      LANGUAGE: "en",
      LC_ALL: "en_US.UTF-8",
      LANG: "en_US.UTF-8"
    };
  }
});

// src/relay/relay-command-env.ts
function getPosixUserInstallBinFallbacks(baseEnv, platform) {
  const home = baseEnv.HOME || (0, import_node_os3.homedir)();
  const bins = [];
  if (baseEnv.PNPM_HOME) {
    bins.push(baseEnv.PNPM_HOME);
  }
  if (home) {
    bins.push(import_node_path4.posix.join(home, ".local", "bin"), import_node_path4.posix.join(home, ".npm-global", "bin"));
    if (platform === "darwin") {
      bins.push(import_node_path4.posix.join(home, "Library", "pnpm"));
    }
  }
  const cargoBin = baseEnv.CARGO_HOME ? import_node_path4.posix.join(baseEnv.CARGO_HOME, "bin") : home ? import_node_path4.posix.join(home, ".cargo", "bin") : null;
  if (cargoBin) {
    bins.push(cargoBin);
  }
  const bunBin = baseEnv.BUN_INSTALL ? import_node_path4.posix.join(baseEnv.BUN_INSTALL, "bin") : home ? import_node_path4.posix.join(home, ".bun", "bin") : null;
  if (bunBin) {
    bins.push(bunBin);
  }
  const denoBin = baseEnv.DENO_INSTALL ? import_node_path4.posix.join(baseEnv.DENO_INSTALL, "bin") : home ? import_node_path4.posix.join(home, ".deno", "bin") : null;
  if (denoBin) {
    bins.push(denoBin);
  }
  const goBin = baseEnv.GOBIN ? baseEnv.GOBIN : baseEnv.GOPATH ? import_node_path4.posix.join(baseEnv.GOPATH, "bin") : home ? import_node_path4.posix.join(home, "go", "bin") : null;
  if (goBin) {
    bins.push(goBin);
  }
  const pnpmHome = baseEnv.PNPM_HOME ? null : baseEnv.XDG_DATA_HOME ? import_node_path4.posix.join(baseEnv.XDG_DATA_HOME, "pnpm") : home ? import_node_path4.posix.join(home, ".local", "share", "pnpm") : null;
  if (pnpmHome) {
    bins.push(pnpmHome);
  }
  const npmPrefix = baseEnv.npm_config_prefix;
  if (npmPrefix) {
    bins.push(import_node_path4.posix.join(npmPrefix, "bin"));
  }
  return bins;
}
function getWindowsUserInstallBinFallbacks(baseEnv) {
  const bins = baseEnv.PNPM_HOME ? [baseEnv.PNPM_HOME] : [];
  if (baseEnv.APPDATA) {
    bins.push(import_node_path4.win32.join(baseEnv.APPDATA, "npm"));
  }
  if (baseEnv.LOCALAPPDATA) {
    bins.push(import_node_path4.win32.join(baseEnv.LOCALAPPDATA, "pnpm"));
  }
  if (baseEnv.CARGO_HOME) {
    bins.push(import_node_path4.win32.join(baseEnv.CARGO_HOME, "bin"));
  }
  if (baseEnv.BUN_INSTALL) {
    bins.push(import_node_path4.win32.join(baseEnv.BUN_INSTALL, "bin"));
  }
  if (baseEnv.DENO_INSTALL) {
    bins.push(import_node_path4.win32.join(baseEnv.DENO_INSTALL, "bin"));
  }
  if (baseEnv.GOBIN) {
    bins.push(baseEnv.GOBIN);
  } else if (baseEnv.GOPATH) {
    bins.push(import_node_path4.win32.join(baseEnv.GOPATH, "bin"));
  }
  if (baseEnv.USERPROFILE) {
    if (!baseEnv.CARGO_HOME) {
      bins.push(import_node_path4.win32.join(baseEnv.USERPROFILE, ".cargo", "bin"));
    }
    if (!baseEnv.BUN_INSTALL) {
      bins.push(import_node_path4.win32.join(baseEnv.USERPROFILE, ".bun", "bin"));
    }
    if (!baseEnv.GOBIN && !baseEnv.GOPATH) {
      bins.push(import_node_path4.win32.join(baseEnv.USERPROFILE, "go", "bin"));
    }
    if (!baseEnv.DENO_INSTALL) {
      bins.push(import_node_path4.win32.join(baseEnv.USERPROFILE, ".deno", "bin"));
    }
  }
  return bins;
}
function getPathKey(env) {
  return env.Path !== void 0 && env.PATH === void 0 ? "Path" : "PATH";
}
function getPathDelimiter(platform) {
  return platform === "win32" ? ";" : ":";
}
function getFallbackSegments(platform, baseEnv) {
  if (platform === "win32") {
    return [...WINDOWS_RELAY_PATH_FALLBACKS, ...getWindowsUserInstallBinFallbacks(baseEnv)];
  }
  return [...POSIX_RELAY_PATH_FALLBACKS, ...getPosixUserInstallBinFallbacks(baseEnv, platform)];
}
function buildRelayCommandEnv(baseEnv = process.env, platform = process.platform) {
  const key = getPathKey(baseEnv);
  const delimiter = getPathDelimiter(platform);
  const segments = new Set((baseEnv[key] ?? "").split(delimiter).filter(Boolean));
  for (const segment of getFallbackSegments(platform, baseEnv)) {
    segments.add(segment);
  }
  return {
    ...baseEnv,
    [key]: [...segments].join(delimiter)
  };
}
function buildRelayGitEnv(baseEnv = process.env, platform = process.platform) {
  return { ...buildRelayCommandEnv(baseEnv, platform), ...UNTRANSLATED_GIT_OUTPUT_ENV };
}
var import_node_os3, import_node_path4, POSIX_RELAY_PATH_FALLBACKS, WINDOWS_RELAY_PATH_FALLBACKS;
var init_relay_command_env = __esm({
  "src/relay/relay-command-env.ts"() {
    "use strict";
    import_node_os3 = require("node:os");
    import_node_path4 = require("node:path");
    init_git_output_locale();
    POSIX_RELAY_PATH_FALLBACKS = ["/usr/local/bin", "/opt/homebrew/bin", "/usr/bin", "/bin"];
    WINDOWS_RELAY_PATH_FALLBACKS = [
      "C:\\Program Files\\Git\\cmd",
      "C:\\Program Files\\Git\\bin",
      "C:\\Windows\\System32",
      "C:\\Windows"
    ];
  }
});

// src/shared/git-discard-path-safety.ts
function isENOENT(error) {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}
function isInsideOrEqual(rootPath, candidatePath) {
  const relativePath = path9.relative(rootPath, candidatePath);
  return relativePath === "" || relativePath !== ".." && !relativePath.startsWith(`..${path9.sep}`) && !path9.isAbsolute(relativePath);
}
async function assertRealPathInsideWorktree(realWorktreePath, candidatePath, originalFilePath) {
  const realCandidatePath = path9.resolve(await (0, import_promises5.realpath)(candidatePath));
  if (!isInsideOrEqual(realWorktreePath, realCandidatePath)) {
    throw new Error(`Path "${originalFilePath}" resolves outside the worktree`);
  }
}
async function assertNearestExistingParentInsideWorktree(realWorktreePath, candidatePath, originalFilePath) {
  let parentPath = path9.dirname(candidatePath);
  while (parentPath !== path9.dirname(parentPath)) {
    try {
      await assertRealPathInsideWorktree(realWorktreePath, parentPath, originalFilePath);
      return;
    } catch (error) {
      if (!isENOENT(error)) {
        throw error;
      }
      parentPath = path9.dirname(parentPath);
    }
  }
  throw new Error(`Path "${originalFilePath}" resolves outside the worktree`);
}
function assertTargetIsWorktreeChild(resolvedWorktreePath, resolvedTarget, originalFilePath) {
  const relativeTarget = path9.relative(resolvedWorktreePath, resolvedTarget);
  if (relativeTarget === "" || relativeTarget === "." || relativeTarget === ".." || relativeTarget.startsWith(`..${path9.sep}`) || path9.isAbsolute(relativeTarget)) {
    throw new Error(`Path "${originalFilePath}" resolves outside the worktree`);
  }
}
async function validateUntrackedDiscardTarget(worktreePath, filePath) {
  const resolvedWorktreePath = path9.resolve(worktreePath);
  const resolvedTarget = path9.resolve(worktreePath, filePath);
  assertTargetIsWorktreeChild(resolvedWorktreePath, resolvedTarget, filePath);
  const realWorktreePath = path9.resolve(await (0, import_promises5.realpath)(worktreePath));
  try {
    const targetStats = await (0, import_promises5.lstat)(resolvedTarget);
    const pathToValidate = targetStats.isSymbolicLink() ? path9.dirname(resolvedTarget) : resolvedTarget;
    await assertRealPathInsideWorktree(realWorktreePath, pathToValidate, filePath);
  } catch (error) {
    if (!isENOENT(error)) {
      throw error;
    }
    await assertNearestExistingParentInsideWorktree(realWorktreePath, resolvedTarget, filePath);
  }
  return resolvedTarget;
}
async function removeSafeUntrackedDiscardTarget(worktreePath, filePath, removePath) {
  await validateUntrackedDiscardTarget(worktreePath, filePath);
  await removePath(filePath);
}
async function removeSafeUntrackedDiscardTargets(worktreePath, filePaths, removePaths, beforeRemove) {
  await Promise.all(
    filePaths.map((filePath) => validateUntrackedDiscardTarget(worktreePath, filePath))
  );
  await beforeRemove?.();
  await Promise.all(
    filePaths.map((filePath) => validateUntrackedDiscardTarget(worktreePath, filePath))
  );
  await removePaths(filePaths);
}
var import_promises5, path9;
var init_git_discard_path_safety = __esm({
  "src/shared/git-discard-path-safety.ts"() {
    "use strict";
    import_promises5 = require("node:fs/promises");
    path9 = __toESM(require("node:path"));
  }
});

// src/shared/git-clone-failure-message.ts
function getGitCloneFailureMessage(stderr, options = {}) {
  let fallbackLine = null;
  const scrubbedStderr = stripCredentialsFromMessage(stderr);
  for (const rawLine of iterateLinesFromEnd2(scrubbedStderr)) {
    const line = stripAnsi(rawLine).trim();
    if (!line) {
      continue;
    }
    fallbackLine ??= line;
    const fatalIndex = line.indexOf("fatal:");
    if (fatalIndex !== -1) {
      return formatGitCloneFailureLine(line.slice(fatalIndex), options);
    }
    const errorIndex = line.indexOf("error:");
    if (errorIndex !== -1) {
      return formatGitCloneFailureLine(line.slice(errorIndex), options);
    }
  }
  return formatGitCloneFailureLine(fallbackLine ?? "unknown error", options);
}
function* iterateLinesFromEnd2(value) {
  let lineEnd = value.length;
  let index = value.length - 1;
  while (index >= 0) {
    const code = value.charCodeAt(index);
    if (code !== 10 && code !== 13) {
      index--;
      continue;
    }
    const delimiterStart = code === 10 && index > 0 && value.charCodeAt(index - 1) === 13 ? index - 1 : index;
    yield value.slice(index + 1, lineEnd);
    lineEnd = delimiterStart;
    index = delimiterStart - 1;
  }
  yield value.slice(0, lineEnd);
}
function stripAnsi(value) {
  return value.replace(new RegExp(`${String.fromCharCode(27)}\\[[0-9;]*m`, "g"), "");
}
function formatGitCloneFailureLine(line, options) {
  const destinationMatch = line.match(
    /^fatal:\s+destination path '([^']+)' already exists and is not an empty directory\.$/
  );
  if (destinationMatch || /repository exists/i.test(line)) {
    const destination = options.clonePath?.trim() || destinationMatch?.[1] || null;
    const target = destination ? `: ${destination}` : "";
    return `Destination already exists and is not empty${target}. Choose a different parent folder, delete the existing folder, or add the existing repository instead.`;
  }
  return line;
}
var init_git_clone_failure_message = __esm({
  "src/shared/git-clone-failure-message.ts"() {
    "use strict";
    init_git_remote_error();
  }
});

// src/shared/git-fork-sync.ts
function parseRemoteHeadBranch(stdout) {
  for (const line of iterateGitOutputLines2(stdout)) {
    const match = /^ref:\s+refs\/heads\/(.+?)\s+HEAD$/.exec(line.trim());
    if (match?.[1]) {
      return match[1];
    }
  }
  return null;
}
function parseAheadBehind(stdout) {
  const [aheadRaw, behindRaw] = getProcessOutputFields(stdout, 2);
  return {
    ahead: Number.parseInt(aheadRaw ?? "0", 10) || 0,
    behind: Number.parseInt(behindRaw ?? "0", 10) || 0
  };
}
async function remoteExists(runGit, remote) {
  const { stdout } = await runGit(["remote"]);
  for (const rawLine of iterateGitOutputLines2(stdout)) {
    if (rawLine.trim() === remote) {
      return true;
    }
  }
  return false;
}
function* iterateGitOutputLines2(output) {
  let lineStart = 0;
  for (let index = 0; index < output.length; index++) {
    const code = output.charCodeAt(index);
    if (code !== 10 && code !== 13) {
      continue;
    }
    yield output.slice(lineStart, index);
    if (code === 13 && output.charCodeAt(index + 1) === 10) {
      index++;
    }
    lineStart = index + 1;
  }
  if (lineStart <= output.length) {
    yield output.slice(lineStart);
  }
}
function cleanGitHubRemotePath(path12) {
  const normalized = path12.replace(/^\/+/, "").replace(/\/+$/, "").replace(/\.git$/i, "");
  const parts = normalized.split("/").filter(Boolean);
  if (parts.length !== 2) {
    return null;
  }
  return parts.join("/").toLowerCase();
}
function parseGitHubRemotePath(remoteUrl) {
  const trimmed = remoteUrl.trim().replace(/^git\+/, "");
  const shorthand = trimmed.match(/^github:([^/].+)$/i);
  if (shorthand) {
    return cleanGitHubRemotePath(shorthand[1]);
  }
  if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed)) {
    const scpLike = trimmed.match(/^(?:[^@/:]+@)?([^:\s/]+):([^\s]+)$/);
    if (scpLike && GITHUB_HOSTS.has(scpLike[1].toLowerCase())) {
      return cleanGitHubRemotePath(scpLike[2]);
    }
  }
  try {
    const url = new URL(trimmed);
    if (!["git:", "http:", "https:", "ssh:"].includes(url.protocol.toLowerCase()) || !GITHUB_HOSTS.has(url.hostname.toLowerCase())) {
      return null;
    }
    return cleanGitHubRemotePath(url.pathname);
  } catch {
    return null;
  }
}
function validateGitForkSyncExpectedUpstream(value, options = {}) {
  if (value === void 0 || value === null) {
    if (options.required) {
      throw new Error("Expected upstream is required.");
    }
    return null;
  }
  if (!value || typeof value !== "object") {
    throw new Error("Invalid expected upstream.");
  }
  const candidate = value;
  const owner = typeof candidate.owner === "string" ? candidate.owner.trim() : "";
  const repo = typeof candidate.repo === "string" ? candidate.repo.trim() : "";
  if (!owner || !repo) {
    throw new Error("Invalid expected upstream.");
  }
  return { owner, repo };
}
async function remoteMatchesExpectedUpstream(runGit, remote, expected) {
  const owner = expected.owner.trim().toLowerCase();
  const repo = expected.repo.trim().toLowerCase();
  if (!owner || !repo) {
    return false;
  }
  try {
    const { stdout } = await runGit(["remote", "get-url", remote]);
    return parseGitHubRemotePath(stdout) === `${owner}/${repo}`;
  } catch {
    return false;
  }
}
async function fetchRemoteBranch(runGit, remote, branchName) {
  try {
    await runGit([
      "fetch",
      "--no-tags",
      "--prune",
      remote,
      `+refs/heads/${branchName}:refs/remotes/${remote}/${branchName}`
    ]);
    return true;
  } catch {
    return false;
  }
}
async function resolveCommit2(runGit, ref) {
  try {
    return (await runGit(["rev-parse", "--verify", `${ref}^{commit}`])).stdout.trim() || null;
  } catch {
    return null;
  }
}
async function resolveRemoteDefaultBranch(runGit, remote) {
  try {
    const { stdout } = await runGit(["ls-remote", "--symref", remote, "HEAD"]);
    const branchName = parseRemoteHeadBranch(stdout);
    if (branchName) {
      return branchName;
    }
  } catch {
  }
  for (const branchName of DEFAULT_BRANCH_FALLBACKS) {
    try {
      await runGit(["rev-parse", "--verify", `refs/remotes/${remote}/${branchName}^{commit}`]);
      return branchName;
    } catch {
    }
  }
  return null;
}
async function isAncestor(runGit, ancestorOid, descendantOid) {
  try {
    await runGit(["merge-base", "--is-ancestor", ancestorOid, descendantOid]);
    return true;
  } catch {
    return false;
  }
}
async function syncForkDefaultBranch(runGit, options = {}) {
  const originRemote = options.originRemote ?? DEFAULT_ORIGIN_REMOTE;
  const upstreamRemote = options.upstreamRemote ?? DEFAULT_UPSTREAM_REMOTE;
  const expectedUpstream = validateGitForkSyncExpectedUpstream(options.expectedUpstream);
  const baseResult = { originRemote, upstreamRemote, ahead: 0, behind: 0 };
  if (!await remoteExists(runGit, originRemote)) {
    return { ...baseResult, status: "blocked", reason: "missing-origin" };
  }
  if (!await remoteExists(runGit, upstreamRemote)) {
    return { ...baseResult, status: "blocked", reason: "missing-upstream" };
  }
  if (expectedUpstream && !await remoteMatchesExpectedUpstream(runGit, upstreamRemote, expectedUpstream)) {
    return { ...baseResult, status: "blocked", reason: "upstream-mismatch" };
  }
  const branchName = await resolveRemoteDefaultBranch(runGit, upstreamRemote);
  if (!branchName) {
    return { ...baseResult, status: "blocked", reason: "missing-upstream-default-branch" };
  }
  await runGit(["check-ref-format", `refs/heads/${branchName}`]);
  const originRef = `refs/remotes/${originRemote}/${branchName}`;
  const upstreamRef = `refs/remotes/${upstreamRemote}/${branchName}`;
  const resultWithBranch = { ...baseResult, branchName };
  if (!await fetchRemoteBranch(runGit, upstreamRemote, branchName)) {
    return { ...resultWithBranch, status: "blocked", reason: "missing-upstream-default-branch" };
  }
  if (!await fetchRemoteBranch(runGit, originRemote, branchName)) {
    return { ...resultWithBranch, status: "blocked", reason: "missing-origin-branch" };
  }
  const upstreamOid = await resolveCommit2(runGit, upstreamRef);
  if (!upstreamOid) {
    return { ...resultWithBranch, status: "blocked", reason: "missing-upstream-default-branch" };
  }
  const originOid = await resolveCommit2(runGit, originRef);
  if (!originOid) {
    return { ...resultWithBranch, status: "blocked", reason: "missing-origin-branch" };
  }
  const counts = parseAheadBehind(
    (await runGit(["rev-list", "--left-right", "--count", `${originOid}...${upstreamOid}`])).stdout
  );
  if (counts.ahead > 0 || !await isAncestor(runGit, originOid, upstreamOid)) {
    return { ...resultWithBranch, ...counts, status: "blocked", reason: "diverged" };
  }
  if (counts.behind === 0) {
    return { ...resultWithBranch, ...counts, status: "up-to-date" };
  }
  await runGit(["push", originRemote, `${upstreamOid}:refs/heads/${branchName}`]);
  await fetchRemoteBranch(runGit, originRemote, branchName);
  return { ...resultWithBranch, ...counts, status: "synced" };
}
var DEFAULT_ORIGIN_REMOTE, DEFAULT_UPSTREAM_REMOTE, DEFAULT_BRANCH_FALLBACKS, GITHUB_HOSTS;
var init_git_fork_sync = __esm({
  "src/shared/git-fork-sync.ts"() {
    "use strict";
    init_process_output_field_scanner();
    DEFAULT_ORIGIN_REMOTE = "origin";
    DEFAULT_UPSTREAM_REMOTE = "upstream";
    DEFAULT_BRANCH_FALLBACKS = ["main", "master"];
    GITHUB_HOSTS = /* @__PURE__ */ new Set(["github.com", "ssh.github.com"]);
  }
});

// src/shared/in-flight-promise-dedupe.ts
function stableInFlightKey(parts) {
  return JSON.stringify(parts);
}
var InFlightPromiseDedupe;
var init_in_flight_promise_dedupe = __esm({
  "src/shared/in-flight-promise-dedupe.ts"() {
    "use strict";
    InFlightPromiseDedupe = class {
      constructor(maxInFlightMs = 3e4) {
        this.maxInFlightMs = maxInFlightMs;
      }
      maxInFlightMs;
      entries = /* @__PURE__ */ new Map();
      run(key, load) {
        const existing = this.entries.get(key);
        if (existing) {
          return existing.promise;
        }
        const promise = Promise.resolve().then(load).finally(() => {
          const entry2 = this.entries.get(key);
          if (entry2?.promise === promise) {
            if (entry2.timeout) {
              clearTimeout(entry2.timeout);
            }
            this.entries.delete(key);
          }
        });
        const entry = {
          promise,
          // Why: renderer diff rows already time out hung loads; drop matching
          // in-flight entries too so retry can issue fresh git work.
          timeout: this.maxInFlightMs > 0 ? setTimeout(() => {
            if (this.entries.get(key)?.promise === promise) {
              this.entries.delete(key);
            }
          }, this.maxInFlightMs) : null
        };
        this.entries.set(key, entry);
        return promise;
      }
      clear() {
        for (const entry of this.entries.values()) {
          if (entry.timeout) {
            clearTimeout(entry.timeout);
          }
        }
        this.entries.clear();
      }
    };
  }
});

// src/shared/git-fetch-auto-maintenance.ts
var GIT_FETCH_SKIP_AUTO_MAINTENANCE_CONFIG_ARGS;
var init_git_fetch_auto_maintenance = __esm({
  "src/shared/git-fetch-auto-maintenance.ts"() {
    "use strict";
    GIT_FETCH_SKIP_AUTO_MAINTENANCE_CONFIG_ARGS = [
      "-c",
      "maintenance.auto=false",
      "-c",
      "maintenance.commit-graph.auto=0",
      "-c",
      "gc.auto=0"
    ];
  }
});

// src/shared/git-capability-cache.ts
var GIT_CAPABILITY_RETRY_INTERVAL_MS, GitCapabilityCache;
var init_git_capability_cache = __esm({
  "src/shared/git-capability-cache.ts"() {
    "use strict";
    GIT_CAPABILITY_RETRY_INTERVAL_MS = 30 * 6e4;
    GitCapabilityCache = class {
      retryAfterByCapability = /* @__PURE__ */ new Map();
      probesByCapability = /* @__PURE__ */ new Map();
      supportedCapabilities = /* @__PURE__ */ new Set();
      shouldTry(capability, nowMs = Date.now()) {
        const retryAfterMs = this.retryAfterByCapability.get(capability);
        if (retryAfterMs === void 0) {
          return true;
        }
        if (nowMs < retryAfterMs) {
          return false;
        }
        this.retryAfterByCapability.delete(capability);
        return true;
      }
      rememberUnsupported(capability, nowMs = Date.now()) {
        this.supportedCapabilities.delete(capability);
        this.retryAfterByCapability.set(capability, nowMs + GIT_CAPABILITY_RETRY_INTERVAL_MS);
      }
      async runWithFallback(capability, runPreferred, runFallback, isUnsupportedError) {
        if (this.supportedCapabilities.has(capability)) {
          return this.runPreferredOrFallback(capability, runPreferred, runFallback, isUnsupportedError);
        }
        if (!this.shouldTry(capability)) {
          return runFallback();
        }
        const inFlightProbe = this.probesByCapability.get(capability);
        if (inFlightProbe) {
          const outcome = await inFlightProbe;
          if (outcome === "unsupported" || !this.shouldTry(capability)) {
            return runFallback();
          }
          return this.runPreferredOrFallback(capability, runPreferred, runFallback, isUnsupportedError);
        }
        let settleProbe;
        const probe = new Promise((resolve8) => {
          settleProbe = resolve8;
        });
        this.probesByCapability.set(capability, probe);
        try {
          return await this.runPreferredOrFallback(
            capability,
            runPreferred,
            runFallback,
            isUnsupportedError,
            settleProbe
          );
        } finally {
          if (this.probesByCapability.get(capability) === probe) {
            this.probesByCapability.delete(capability);
          }
        }
      }
      clear() {
        this.retryAfterByCapability.clear();
        this.probesByCapability.clear();
        this.supportedCapabilities.clear();
      }
      async runPreferredOrFallback(capability, runPreferred, runFallback, isUnsupportedError, settleProbe) {
        try {
          const result = await runPreferred();
          const outcome = this.retryAfterByCapability.has(capability) ? "unsupported" : "supported";
          if (outcome === "supported") {
            this.supportedCapabilities.add(capability);
          }
          settleProbe?.(outcome);
          return result;
        } catch (error) {
          if (!isUnsupportedError(error)) {
            settleProbe?.("unknown");
            throw error;
          }
          this.rememberUnsupported(capability);
          settleProbe?.("unsupported");
          return runFallback();
        }
      }
    };
  }
});

// src/relay/protocol.ts
var RELAY_VERSION2, RELAY_SENTINEL2, MAX_MESSAGE_SIZE2, STREAM_CHUNK_SIZE2, STREAM_ACK_WINDOW_CHUNKS, STREAM_ACK_STALL_RECHECK_MS, GIT_RESPONSE_STREAM_THRESHOLD2, GIT_RESPONSE_CHUNK_SIZE2;
var init_protocol = __esm({
  "src/relay/protocol.ts"() {
    "use strict";
    RELAY_VERSION2 = "0.1.0";
    RELAY_SENTINEL2 = `ORCA-RELAY v${RELAY_VERSION2} READY
`;
    MAX_MESSAGE_SIZE2 = 16 * 1024 * 1024;
    STREAM_CHUNK_SIZE2 = 256 * 1024;
    STREAM_ACK_WINDOW_CHUNKS = 4;
    STREAM_ACK_STALL_RECHECK_MS = 1e3;
    GIT_RESPONSE_STREAM_THRESHOLD2 = 256 * 1024;
    GIT_RESPONSE_CHUNK_SIZE2 = 128 * 1024;
  }
});

// src/relay/git-response-stream.ts
function encodeChunks(payload) {
  const chunks = [];
  for (let offset = 0; offset < payload.length; offset += GIT_RESPONSE_CHUNK_SIZE2) {
    chunks.push(payload.subarray(offset, offset + GIT_RESPONSE_CHUNK_SIZE2).toString("base64"));
  }
  return chunks;
}
var GitResponseStreamRegistry;
var init_git_response_stream = __esm({
  "src/relay/git-response-stream.ts"() {
    "use strict";
    init_protocol();
    GitResponseStreamRegistry = class {
      streams = /* @__PURE__ */ new Map();
      nextId = 1;
      register(ownerClientId) {
        const streamId = this.nextId++;
        this.streams.set(streamId, {
          ownerClientId,
          aborted: false,
          ackedThroughSeq: -1,
          ackWaiters: /* @__PURE__ */ new Set()
        });
        return streamId;
      }
      recordAck(streamId, seq, clientId) {
        const entry = this.streams.get(streamId);
        if (!entry || entry.ownerClientId !== clientId || typeof seq !== "number" || !Number.isFinite(seq)) {
          return;
        }
        if (seq > entry.ackedThroughSeq) {
          entry.ackedThroughSeq = seq;
        }
        this.wake(entry);
      }
      abort(streamId, clientId) {
        const entry = this.streams.get(streamId);
        if (entry?.ownerClientId === clientId) {
          entry.aborted = true;
          this.wake(entry);
        }
      }
      /** Wake every parked pump so it re-checks staleness — used when a client
       * detaches and its acks will never arrive. */
      wakeAll() {
        for (const entry of this.streams.values()) {
          this.wake(entry);
        }
      }
      wake(entry) {
        for (const waiter of Array.from(entry.ackWaiters)) {
          waiter();
        }
      }
      waitForAck(streamId) {
        const entry = this.streams.get(streamId);
        if (!entry || entry.aborted) {
          return Promise.resolve();
        }
        return new Promise((resolve8) => {
          let settled = false;
          const finish = () => {
            if (settled) {
              return;
            }
            settled = true;
            clearTimeout(timer);
            entry.ackWaiters.delete(finish);
            resolve8();
          };
          const timer = setTimeout(finish, STREAM_ACK_STALL_RECHECK_MS);
          timer.unref?.();
          entry.ackWaiters.add(finish);
        });
      }
      /**
       * Register a stream for `payload`, kick off the bulk-lane pump on a later
       * task (so the sentinel response reaches the client first), and return the
       * sentinel marker to send as the RPC result.
       */
      startStream(payload, dispatcher, context) {
        const streamId = this.register(context.clientId);
        const chunks = encodeChunks(payload);
        setImmediate(() => {
          void this.pump(streamId, chunks, dispatcher, context);
        });
        return {
          __orcaGitResponseStream: { streamId, totalBytes: payload.length, chunkCount: chunks.length }
        };
      }
      async pump(streamId, chunks, dispatcher, context) {
        const entry = this.streams.get(streamId);
        if (!entry) {
          return;
        }
        const clientId = context.clientId;
        let seq = 0;
        let endReason = "end";
        try {
          for (seq = 0; seq < chunks.length; seq += 1) {
            if (context.isStale()) {
              endReason = "stale";
              break;
            }
            if (entry.aborted) {
              endReason = "aborted";
              break;
            }
            while (seq - entry.ackedThroughSeq > STREAM_ACK_WINDOW_CHUNKS && !context.isStale() && !entry.aborted) {
              await this.waitForAck(streamId);
            }
            if (context.isStale()) {
              endReason = "stale";
              break;
            }
            if (entry.aborted) {
              endReason = "aborted";
              break;
            }
            await dispatcher.notifyBulk(
              "git.responseChunk",
              { streamId, seq, data: chunks[seq] },
              {
                clientId
              }
            );
          }
          if (endReason === "end") {
            await dispatcher.notifyBulk("git.responseEnd", { streamId }, { clientId });
          }
        } catch (err) {
          if (!context.isStale() && !entry.aborted) {
            try {
              await dispatcher.notifyBulk(
                "git.responseError",
                {
                  streamId,
                  message: err instanceof Error ? err.message : String(err)
                },
                { clientId }
              );
            } catch {
            }
          }
        } finally {
          this.streams.delete(streamId);
        }
      }
      disposeAll() {
        for (const entry of this.streams.values()) {
          entry.aborted = true;
          this.wake(entry);
        }
        this.streams.clear();
      }
    };
  }
});

// src/shared/subprocess-stdin-write.ts
function endSubprocessStdin(stdin, input) {
  if (!stdin) {
    return;
  }
  stdin.once("error", () => {
  });
  stdin.end(input);
}
var init_subprocess_stdin_write = __esm({
  "src/shared/subprocess-stdin-write.ts"() {
    "use strict";
  }
});

// src/relay/git-handler.ts
var git_handler_exports = {};
__export(git_handler_exports, {
  GitHandler: () => GitHandler,
  parseWorktreePorcelain: () => parseWorktreePorcelain,
  validateWorktreePath: () => validateWorktreePath
});
function resolveSubmoduleStatusArea(params) {
  if (params.area === "staged" || params.area === "unstaged" || params.area === "untracked") {
    return params.area;
  }
  return "unstaged";
}
function isWindowsAbsolutePath3(value) {
  return /^[A-Za-z]:[\\/]/.test(value) || value.startsWith("\\\\");
}
function resolveRelayPath(repoPath, value) {
  if (path10.posix.isAbsolute(value) || path10.win32.isAbsolute(value)) {
    return value;
  }
  return isWindowsAbsolutePath3(repoPath) ? path10.win32.resolve(repoPath, value) : path10.posix.resolve(repoPath, value);
}
function parseRelayRepoLocation(repoPath, output) {
  const lines = output.split("\n").map((line) => line.endsWith("\r") ? line.slice(0, -1) : line).filter((line) => line.length > 0 && !line.startsWith("-"));
  if (lines.length < 2) {
    return void 0;
  }
  const [topLevel, commonDir] = lines.slice(-2);
  return {
    topLevel: resolveRelayPath(repoPath, topLevel),
    commonDir: resolveRelayPath(repoPath, commonDir)
  };
}
function execFileWithStdin(command, args, options, stdin) {
  return new Promise((resolve8, reject) => {
    let settled = false;
    const finish = (error, stdout = "", stderr = "") => {
      if (settled) {
        return;
      }
      settled = true;
      if (error) {
        reject(Object.assign(error, { stdout, stderr }));
        return;
      }
      resolve8({ stdout: String(stdout), stderr: String(stderr) });
    };
    const child = (0, import_node_child_process2.execFile)(command, args, options, (error, stdout, stderr) => {
      if (error) {
        finish(error, stdout, stderr);
        return;
      }
      finish(null, stdout, stderr);
    });
    child.once("error", (error) => finish(error));
    endSubprocessStdin(child.stdin, stdin);
  });
}
function validateWorktreePath(args, workDir) {
  if (args[0] !== "worktree" || args[1] !== "add" || !args[2]) {return;}
  const rawPath = args[2];
  const resolved = rawPath.startsWith("/") ? path10.resolve(rawPath) : path10.resolve(path10.join(workDir, rawPath));
  if (resolved.includes("\0")) {
    throw Object.assign(
      new Error("GIT_WORKTREE_PATH_INVALID: null bytes in path"),
      { code: "GIT_WORKTREE_PATH_INVALID" }
    );
  }
  const parentDir = path10.dirname(workDir);
  const allowedRoots = [workDir, parentDir, "/tmp", "/var/tmp"];
  const isAllowed = allowedRoots.some(
    (root) => resolved === root || resolved.startsWith(`${root  }/`)
  );
  if (!isAllowed) {
    throw Object.assign(
      new Error(`GIT_WORKTREE_PATH_NOT_ALLOWED: "${resolved}" is outside allowed roots`),
      { code: "GIT_WORKTREE_PATH_NOT_ALLOWED", path: resolved }
    );
  }
}
function parseWorktreePorcelain(stdout) {
  const worktrees = [];
  let current = null;
  const flush = () => {
    if (current?.path !== void 0) {
      worktrees.push({
        path: current.path ?? "",
        head: current.head ?? "",
        branch: current.branch ?? "",
        bare: current.bare ?? false,
        detached: current.detached ?? false,
        prunable: current.prunable ?? false,
        locked: current.locked ?? false,
        lockedReason: current.lockedReason
      });
    }
    current = null;
  };
  for (const rawLine of stdout.split("\n")) {
    const line = rawLine.trim();
    if (line === "") {
      flush();
      continue;
    }
    if (line.startsWith("worktree ")) {
      flush();
      current = { path: line.slice("worktree ".length) };
    } else if (line.startsWith("HEAD ") && current) {
      current.head = line.slice("HEAD ".length);
    } else if (line.startsWith("branch ") && current) {
      current.branch = line.slice("branch ".length).replace("refs/heads/", "");
    } else if (line === "bare" && current) {
      current.bare = true;
    } else if (line === "detached" && current) {
      current.detached = true;
    } else if (line.startsWith("prunable") && current) {
      current.prunable = true;
    } else if (line === "locked" && current) {
      current.locked = true;
    } else if (line.startsWith("locked ") && current) {
      current.locked = true;
      current.lockedReason = line.slice("locked ".length);
    }
  }
  flush();
  return worktrees;
}
var import_node_child_process2, import_node_util, path10, execFileAsync, MAX_GIT_BUFFER, BULK_CHUNK_SIZE, GitHandler;
var init_git_handler = __esm({
  "src/relay/git-handler.ts"() {
    "use strict";
    import_node_child_process2 = require("node:child_process");
    import_node_util = require("node:util");
    path10 = __toESM(require("node:path"));
    init_context();
    init_git_handler_utils();
    init_git_uncommitted_line_stats();
    init_git_handler_ops();
    init_git_handler_submodule_ops();
    init_git_handler_commit_diff_ops();
    init_git_handler_worktree_ops();
    init_git_handler_branch_cleanup();
    init_git_handler_local_base_ref_refresh();
    init_git_exec_mutation();
    init_git_handler_status_ops();
    init_git_handler_check_ignore();
    init_git_handler_push_target();
    init_git_remote_error();
    init_git_upstream_status();
    init_git_push_target_validation();
    init_git_publish_target_status();
    init_git_rebase_source();
    init_git_effective_upstream();
    init_git_history();
    init_relay_command_env();
    init_git_discard_path_safety();
    init_git_clone_failure_message();
    init_git_fork_sync();
    init_in_flight_promise_dedupe();
    init_git_fetch_auto_maintenance();
    init_git_capability_cache();
    init_git_worktree_command_capabilities();
    init_git_response_stream();
    init_protocol();
    init_subprocess_stdin_write();
    execFileAsync = (0, import_node_util.promisify)(import_node_child_process2.execFile);
    MAX_GIT_BUFFER = 10 * 1024 * 1024;
    BULK_CHUNK_SIZE = 100;
    GitHandler = class {
      dispatcher;
      gitDiffReadDedupe = new InFlightPromiseDedupe();
      gitCapabilities = new GitCapabilityCache();
      // Why: large diff/exec responses are chunked onto the bulk lane so they do
      // not head-of-line-block interactive pty.data echo on the shared SSH channel.
      responseStreams = new GitResponseStreamRegistry();
      // Why: configured submodule paths change rarely; an instance-level TTL cache
      // avoids re-reading `.gitmodules` on every diff click over SSH, and being
      // per-instance it stays bound to the connection lifecycle (no cross-test leak).
      submodulePathsCache = createSubmodulePathsCache();
      // Why: RelayContext is accepted for protocol back-compat (see
      // docs/relay-fs-allowlist-removal.md) but no longer consulted on git ops.
      constructor(dispatcher, _context) {
        this.dispatcher = dispatcher;
        this.registerHandlers();
        this.dispatcher.onClientDetached?.(() => this.responseStreams.wakeAll());
      }
      dispose() {
        this.responseStreams.disposeAll();
        this.clearGitMutationReadCaches();
      }
      registerHandlers() {
        this.dispatcher.onRequest("git.status", (p) => this.getStatus(p));
        this.dispatcher.onRequest("git.submoduleStatus", (p) => this.getSubmoduleStatus(p));
        this.dispatcher.onRequest("git.checkIgnored", (p) => this.checkIgnored(p));
        this.dispatcher.onRequest("git.history", (p) => this.history(p));
        this.dispatcher.onRequest("git.commit", (p) => this.commit(p));
        this.dispatcher.onRequest("git.diff", (p, context) => this.getDiff(p, context));
        this.dispatcher.onRequest("git.stage", (p) => this.stage(p));
        this.dispatcher.onRequest("git.unstage", (p) => this.unstage(p));
        this.dispatcher.onRequest("git.bulkStage", (p) => this.bulkStage(p));
        this.dispatcher.onRequest("git.bulkUnstage", (p) => this.bulkUnstage(p));
        this.dispatcher.onRequest("git.abortMerge", (p) => this.abortMerge(p));
        this.dispatcher.onRequest("git.abortRebase", (p) => this.abortRebase(p));
        this.dispatcher.onRequest("git.checkout", (p) => this.checkout(p));
        this.dispatcher.onRequest("git.localBranches", (p) => this.localBranches(p));
        this.dispatcher.onRequest("git.discard", (p) => this.discard(p));
        this.dispatcher.onRequest("git.bulkDiscard", (p) => this.bulkDiscard(p));
        this.dispatcher.onRequest("git.conflictOperation", (p) => this.conflictOperation(p));
        this.dispatcher.onRequest("git.branchCompare", (p) => this.branchCompare(p));
        this.dispatcher.onRequest("git.commitCompare", (p) => this.commitCompare(p));
        this.dispatcher.onRequest("git.upstreamStatus", (p) => this.upstreamStatus(p));
        this.dispatcher.onRequest("git.fetch", (p) => this.fetch(p));
        this.dispatcher.onRequest("git.forkSync", (p, context) => this.forkSync(p, context));
        this.dispatcher.onRequest("git.fetchRemoteTrackingRef", (p) => this.fetchRemoteTrackingRef(p));
        this.dispatcher.onRequest(
          "git.fetchGitLabMergeRequestHead",
          (p) => this.fetchGitLabMergeRequestHead(p)
        );
        this.dispatcher.onRequest("git.push", (p) => this.push(p));
        this.dispatcher.onRequest("git.pull", (p) => this.pull(p));
        this.dispatcher.onRequest("git.fastForward", (p) => this.fastForward(p));
        this.dispatcher.onRequest("git.rebaseFromBase", (p) => this.rebaseFromBase(p));
        this.dispatcher.onRequest("git.branchDiff", (p, context) => this.branchDiff(p, context));
        this.dispatcher.onRequest("git.commitDiff", (p, context) => this.commitDiff(p, context));
        this.dispatcher.onRequest("git.listWorktrees", (p, context) => this.listWorktrees(p, context));
        this.dispatcher.onRequest("git.addWorktree", (p) => this.addWorktree(p));
        this.dispatcher.onRequest("git.removeWorktree", (p) => this.removeWorktree(p));
        this.dispatcher.onRequest("git.worktreeIsClean", (p) => this.worktreeIsClean(p));
        this.dispatcher.onRequest(
          "git.refreshLocalBaseRefForWorktreeCreate",
          (p) => this.refreshLocalBaseRefForWorktreeCreate(p)
        );
        this.dispatcher.onRequest("git.renameCurrentBranch", (p) => this.renameCurrentBranch(p));
        this.dispatcher.onRequest(
          "git.forceDeletePreservedBranch",
          (p) => this.forceDeletePreservedBranch(p)
        );
        this.dispatcher.onRequest("git.exec", (p, context) => this.exec(p, context));
        this.dispatcher.onRequest("git.clone", (p, context) => this.clone(p, context));
        this.dispatcher.onRequest("git.isGitRepo", (p) => this.isGitRepo(p));
        this.dispatcher.onNotification("git.responseAck", (p, context) => this.responseAck(p, context));
        this.dispatcher.onNotification(
          "git.cancelResponseStream",
          (p, context) => this.cancelResponseStream(p, context)
        );
      }
      responseAck(params, context) {
        const streamId = params.streamId;
        const seq = params.seq;
        if (typeof streamId === "number" && typeof seq === "number") {
          this.responseStreams.recordAck(streamId, seq, context.clientId);
        }
      }
      cancelResponseStream(params, context) {
        const streamId = params.streamId;
        if (typeof streamId === "number") {
          this.responseStreams.abort(streamId, context.clientId);
        }
      }
      // Why: when the client opted into response streaming and the serialized result
      // exceeds the threshold, chunk it onto the bulk lane and return a small
      // sentinel as the RPC result. Old clients omit the flag (single-frame, as
      // today); old relays never call this, so a new client falls back to the plain
      // result they return.
      maybeStreamResponse(result, params, context) {
        if (params.__streamResponse !== true || !context) {
          return result;
        }
        const payload = Buffer.from(JSON.stringify(result ?? null), "utf-8");
        if (payload.length <= GIT_RESPONSE_STREAM_THRESHOLD2) {
          return result;
        }
        return this.responseStreams.startStream(payload, this.dispatcher, context);
      }
      clearGitMutationReadCaches() {
        this.gitDiffReadDedupe.clear();
        clearSubmodulePathsCache(this.submodulePathsCache);
      }
      async runWithGitReadCacheClear(run) {
        this.clearGitMutationReadCaches();
        try {
          return await run();
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async git(args, cwd, opts) {
        const env = buildRelayGitEnv();
        if (opts?.disableOptionalLocks) {
          env.GIT_OPTIONAL_LOCKS = "0";
        }
        if (opts?.nonInteractive) {
          env.GIT_TERMINAL_PROMPT = "0";
          env.GIT_ASKPASS = "";
          env.SSH_ASKPASS = "";
          env.GIT_SSH_COMMAND ??= "ssh -o BatchMode=yes";
        }
        const execOptions = {
          cwd: expandTilde(cwd),
          env,
          encoding: "utf-8",
          maxBuffer: opts?.maxBuffer ?? MAX_GIT_BUFFER,
          timeout: opts?.timeout,
          signal: opts?.signal
        };
        if (opts?.stdin !== void 0) {
          return execFileWithStdin("git", args, execOptions, opts.stdin);
        }
        const { stdout, stderr } = await execFileAsync("git", args, execOptions);
        return { stdout: String(stdout), stderr: String(stderr) };
      }
      async gitBuffer(args, cwd) {
        const { stdout } = await execFileAsync("git", args, {
          cwd,
          env: buildRelayGitEnv(),
          encoding: "buffer",
          maxBuffer: MAX_GIT_BUFFER
        });
        return stdout;
      }
      async getStatus(params) {
        this.gitDiffReadDedupe.clear();
        return getStatusOp(this.git.bind(this), params);
      }
      // Why: the parent status only lists a single gitlink row per submodule. The
      // renderer fetches inner per-file changes on demand by running a plain status
      // inside the submodule's own worktree. Reject paths escaping the worktree to
      // match the diff handler's traversal guard.
      async getSubmoduleStatus(params) {
        const worktreePath = params.worktreePath;
        const submodulePath = params.submodulePath;
        const area = resolveSubmoduleStatusArea(params);
        const staged = area === "staged";
        const resolved = resolveSubmoduleWorktreePath(worktreePath, submodulePath);
        const workingResult = await getStatusOp(this.git.bind(this), {
          ...params,
          worktreePath: resolved
        });
        const { fromOid, toOid } = await resolveSubmoduleCommitRange(
          this.git.bind(this),
          worktreePath,
          submodulePath,
          staged
        );
        if (fromOid && toOid && fromOid !== toOid) {
          const rangeEntries = await computeSubmoduleRangeEntries(
            this.git.bind(this),
            resolved,
            fromOid,
            toOid
          );
          if (staged) {
            return { ...workingResult, entries: rangeEntries };
          }
          const rangePaths = new Set(rangeEntries.map((entry) => entry.path));
          const entries = [
            ...rangeEntries,
            ...workingResult.entries.filter((entry) => !rangePaths.has(entry.path))
          ];
          return { ...workingResult, entries };
        }
        if (staged) {
          return { ...workingResult, entries: [] };
        }
        return workingResult;
      }
      async checkIgnored(params) {
        return checkIgnoredPathsOp(this.git.bind(this), params);
      }
      async history(params) {
        const worktreePath = params.worktreePath;
        return loadGitHistoryFromExecutor(this.git.bind(this), worktreePath, {
          limit: typeof params.limit === "number" ? params.limit : void 0,
          baseRef: typeof params.baseRef === "string" ? params.baseRef : null
        });
      }
      async getDiff(params, context) {
        const worktreePath = params.worktreePath;
        const filePath = params.filePath;
        const resolved = path10.resolve(worktreePath, filePath);
        const rel = path10.relative(path10.resolve(worktreePath), resolved);
        if (rel === ".." || rel.startsWith(`..${path10.sep}`) || path10.isAbsolute(rel)) {
          throw new Error(`Path "${filePath}" resolves outside the worktree`);
        }
        const staged = params.staged;
        const compareAgainstHead = params.compareAgainstHead;
        const result = await this.gitDiffReadDedupe.run(
          stableInFlightKey(["diff", worktreePath, filePath, staged, compareAgainstHead]),
          async () => {
            const submodulePaths = await listSubmodulePathsCached(
              this.git.bind(this),
              worktreePath,
              this.submodulePathsCache
            );
            if (submodulePaths.length > 0) {
              const matchedSubmodule = findContainingSubmodule(submodulePaths, filePath);
              if (matchedSubmodule) {
                const normalizedFilePath = filePath.replace(/\\/g, "/").replace(/\/+$/, "");
                if (normalizedFilePath === matchedSubmodule) {
                  return computeSubmodulePointerDiff(
                    this.git.bind(this),
                    worktreePath,
                    matchedSubmodule,
                    staged,
                    compareAgainstHead
                  );
                }
                const submoduleWorktreePath = resolveSubmoduleWorktreePath(
                  worktreePath,
                  matchedSubmodule
                );
                const innerPath = normalizedFilePath.slice(matchedSubmodule.length + 1);
                const { fromOid, toOid } = await resolveSubmoduleCommitRange(
                  this.git.bind(this),
                  worktreePath,
                  matchedSubmodule,
                  staged
                );
                if (fromOid && toOid && fromOid !== toOid) {
                  return buildSubmoduleInnerCommitRangeDiff(
                    this.gitBuffer.bind(this),
                    submoduleWorktreePath,
                    innerPath,
                    fromOid,
                    toOid
                  );
                }
                return computeDiff(
                  this.gitBuffer.bind(this),
                  submoduleWorktreePath,
                  innerPath,
                  staged,
                  compareAgainstHead
                );
              }
            }
            return computeDiff(
              this.gitBuffer.bind(this),
              worktreePath,
              filePath,
              staged,
              compareAgainstHead
            );
          }
        );
        return this.maybeStreamResponse(result, params, context);
      }
      async stage(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePath = params.filePath;
        try {
          await this.git(["add", "--", this.literalPathspec(filePath)], worktreePath);
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async commit(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const message = params.message;
        try {
          return await commitChangesRelay(this.git.bind(this), worktreePath, message);
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async unstage(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePath = params.filePath;
        try {
          await this.git(["restore", "--staged", "--", this.literalPathspec(filePath)], worktreePath);
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async bulkStage(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePaths = params.filePaths;
        try {
          for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
            const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE);
            await this.git(
              ["add", "--", ...chunk.map((filePath) => this.literalPathspec(filePath))],
              worktreePath
            );
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async bulkUnstage(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePaths = params.filePaths;
        try {
          for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
            const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE);
            await this.git(
              ["restore", "--staged", "--", ...chunk.map((filePath) => this.literalPathspec(filePath))],
              worktreePath
            );
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async abortMerge(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        try {
          await this.git(["merge", "--abort"], worktreePath);
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async abortRebase(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        try {
          await this.git(["rebase", "--abort"], worktreePath);
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async checkout(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const branch = params.branch;
        if (typeof branch !== "string" || branch.length === 0 || branch.startsWith("-")) {
          throw new Error("invalid_branch_name");
        }
        try {
          await this.git(["checkout", branch, "--"], worktreePath);
          return { ok: true, branch };
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async localBranches(params) {
        const worktreePath = params.worktreePath;
        const { stdout } = await this.git(
          ["for-each-ref", "--format=%(HEAD)%09%(refname:short)", "refs/heads/"],
          worktreePath
        );
        let current = null;
        const branches = [];
        for (const line of stdout.split("\n")) {
          if (line.length === 0) {
            continue;
          }
          const [marker, name] = line.split("	");
          if (!name) {
            continue;
          }
          if (marker === "*") {
            current = name;
          }
          branches.push(name);
        }
        branches.sort((a, b) => a === current ? -1 : b === current ? 1 : 0);
        return { current, branches };
      }
      normalizeGitPathForCompare(filePath) {
        return filePath.replace(/\\/g, "/").replace(/\/+$/, "");
      }
      isTrackedPathSpec(filePath, trackedPaths) {
        const normalized = this.normalizeGitPathForCompare(filePath);
        return trackedPaths.some((trackedPath) => {
          const normalizedTracked = this.normalizeGitPathForCompare(trackedPath);
          return normalizedTracked === normalized || normalizedTracked.startsWith(`${normalized}/`);
        });
      }
      assertInWorktree(worktreePath, filePath) {
        const resolved = path10.resolve(worktreePath, filePath);
        const rel = path10.relative(path10.resolve(worktreePath), resolved);
        if (!rel || rel === "." || rel === ".." || rel.startsWith(`..${path10.sep}`) || path10.isAbsolute(rel)) {
          throw new Error(`Path "${filePath}" resolves outside the worktree`);
        }
        return resolved;
      }
      async discard(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePath = params.filePath;
        try {
          this.assertInWorktree(worktreePath, filePath);
          let tracked = false;
          try {
            await this.git(
              ["ls-files", "--error-unmatch", "--", this.literalPathspec(filePath)],
              worktreePath
            );
            tracked = true;
          } catch {
          }
          if (tracked) {
            await this.git(
              ["restore", "--worktree", "--source=HEAD", "--", this.literalPathspec(filePath)],
              worktreePath
            );
            return;
          }
          await removeSafeUntrackedDiscardTarget(
            worktreePath,
            filePath,
            (targetPath) => this.cleanUntrackedPaths(worktreePath, [targetPath])
          );
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async bulkDiscard(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const filePaths = params.filePaths;
        if (filePaths.length === 0) {
          return;
        }
        try {
          for (const filePath of filePaths) {
            this.assertInWorktree(worktreePath, filePath);
          }
          const trackedPathSpecs = [];
          for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
            const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE);
            const { stdout } = await this.git(
              ["ls-files", "-z", "--", ...chunk.map((p) => this.literalPathspec(p))],
              worktreePath
            );
            for (const trackedPathSpec of stdout.split("\0")) {
              if (trackedPathSpec) {
                trackedPathSpecs.push(trackedPathSpec);
              }
            }
          }
          const trackedPaths = filePaths.filter(
            (filePath) => this.isTrackedPathSpec(filePath, trackedPathSpecs)
          );
          const untrackedPaths = filePaths.filter(
            (filePath) => !this.isTrackedPathSpec(filePath, trackedPathSpecs)
          );
          await removeSafeUntrackedDiscardTargets(
            worktreePath,
            untrackedPaths,
            (targetPaths) => this.cleanUntrackedPaths(worktreePath, targetPaths),
            async () => {
              for (let i = 0; i < trackedPaths.length; i += BULK_CHUNK_SIZE) {
                const chunk = trackedPaths.slice(i, i + BULK_CHUNK_SIZE);
                await this.git(
                  [
                    "restore",
                    "--worktree",
                    "--source=HEAD",
                    "--",
                    ...chunk.map((p) => this.literalPathspec(p))
                  ],
                  worktreePath
                );
              }
            }
          );
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      literalPathspec(filePath) {
        return `:(literal)${filePath}`;
      }
      async cleanUntrackedPaths(worktreePath, filePaths) {
        for (let i = 0; i < filePaths.length; i += BULK_CHUNK_SIZE) {
          const chunk = filePaths.slice(i, i + BULK_CHUNK_SIZE);
          if (chunk.length > 0) {
            await this.git(
              ["clean", "-ffdx", "--", ...chunk.map((p) => this.literalPathspec(p))],
              worktreePath
            );
          }
        }
      }
      async conflictOperation(params) {
        const worktreePath = params.worktreePath;
        return detectConflictOperation(worktreePath);
      }
      async branchCompare(params) {
        const worktreePath = params.worktreePath;
        const baseRef = params.baseRef;
        if (baseRef.startsWith("-")) {
          throw new Error('Base ref must not start with "-"');
        }
        const gitBound = this.git.bind(this);
        return branchCompare(gitBound, worktreePath, baseRef, async (mergeBase, headOid) => {
          const { stdout } = await gitBound(
            ["-c", "core.quotePath=false", "diff", "--name-status", "-M", "-C", mergeBase, headOid],
            worktreePath
          );
          const { stdout: numstat } = await gitBound(
            ["-c", "core.quotePath=false", "diff", "--numstat", "-M", "-C", mergeBase, headOid],
            worktreePath
          );
          return parseBranchDiff(stdout, parseNumstat(numstat));
        });
      }
      async commitCompare(params) {
        const worktreePath = params.worktreePath;
        const commitId = params.commitId;
        return commitCompare(this.git.bind(this), worktreePath, commitId);
      }
      async upstreamStatus(params) {
        const worktreePath = params.worktreePath;
        try {
          if (params.pushTarget !== void 0) {
            assertGitPushTargetShape(params.pushTarget);
            const pushTarget = params.pushTarget;
            await this.git(["check-ref-format", "--branch", pushTarget.branchName], worktreePath);
            return await getPublishTargetStatus(
              ((args) => this.git(args, worktreePath)),
              pushTarget,
              (upstreamName) => this.getBehindCommitsArePatchEquivalent(worktreePath, upstreamName)
            );
          }
          return await getEffectiveGitUpstreamStatus(
            (args) => this.git(args, worktreePath),
            (upstreamName) => this.getBehindCommitsArePatchEquivalent(worktreePath, upstreamName)
          );
        } catch (error) {
          if (isNoUpstreamError(error)) {
            return { hasUpstream: false, ahead: 0, behind: 0 };
          }
          throw new Error(normalizeGitErrorMessage(error, "upstream"));
        }
      }
      async getBehindCommitsArePatchEquivalent(worktreePath, upstreamName) {
        try {
          const { stdout } = await this.git(
            ["log", "--oneline", "--cherry-mark", "--right-only", `HEAD...${upstreamName}`, "--"],
            worktreePath
          );
          return upstreamOnlyCommitsArePatchEquivalent(stdout);
        } catch {
          return false;
        }
      }
      async fetch(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        try {
          try {
            if (params.pushTarget !== void 0) {
              assertGitPushTargetShape(params.pushTarget);
              const pushTarget = params.pushTarget;
              await this.git(["check-ref-format", "--branch", pushTarget.branchName], worktreePath);
              await this.git(["fetch", "--prune", pushTarget.remoteName], worktreePath);
              return;
            }
            await this.git(["fetch", "--prune"], worktreePath);
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "fetch"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async forkSync(params, context) {
        return this.runWithGitReadCacheClear(async () => {
          const worktreePath = params.worktreePath;
          const expectedUpstream = validateGitForkSyncExpectedUpstream(params.expectedUpstream, {
            required: true
          });
          const controller = new AbortController();
          const abortFromContext = () => controller.abort();
          if (context?.signal?.aborted) {
            controller.abort();
          } else {
            context?.signal?.addEventListener("abort", abortFromContext, { once: true });
          }
          const timeout = setTimeout(() => controller.abort(), 6e4);
          try {
            return await syncForkDefaultBranch(
              (args) => this.git(args, worktreePath, {
                nonInteractive: true,
                signal: controller.signal
              }),
              { expectedUpstream }
            );
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "push"));
          } finally {
            clearTimeout(timeout);
            context?.signal?.removeEventListener("abort", abortFromContext);
          }
        });
      }
      async fetchRemoteTrackingRef(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const remote = params.remote;
        const branch = params.branch;
        const ref = params.ref;
        const skipAutoMaintenance = params.skipAutoMaintenance;
        try {
          if (typeof remote !== "string" || typeof branch !== "string" || typeof ref !== "string") {
            throw new Error("Invalid remote-tracking fetch request.");
          }
          if (skipAutoMaintenance !== void 0 && typeof skipAutoMaintenance !== "boolean") {
            throw new Error("Invalid remote-tracking fetch maintenance option.");
          }
          if (remote.startsWith("-") || branch.startsWith("-")) {
            throw new Error('Remote-tracking fetch inputs must not start with "-".');
          }
          if (ref !== `refs/remotes/${remote}/${branch}`) {
            throw new Error("Remote-tracking ref does not match the requested remote and branch.");
          }
          try {
            const { stdout } = await this.git(["remote"], worktreePath);
            const remotes = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
            if (!remotes.includes(remote)) {
              throw new Error(`Remote "${remote}" is not configured.`);
            }
            await this.git(["check-ref-format", `refs/heads/${branch}`], worktreePath);
            await this.git(["check-ref-format", ref], worktreePath);
            await this.git(
              [
                ...skipAutoMaintenance ? GIT_FETCH_SKIP_AUTO_MAINTENANCE_CONFIG_ARGS : [],
                "fetch",
                "--no-tags",
                remote,
                `+refs/heads/${branch}:${ref}`
              ],
              worktreePath
            );
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "fetch"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async fetchGitLabMergeRequestHead(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const remote = params.remote;
        const mrIid = params.mrIid;
        try {
          if (typeof remote !== "string") {
            throw new Error("Invalid GitLab merge request fetch request.");
          }
          if (typeof mrIid !== "number" || !Number.isSafeInteger(mrIid) || mrIid <= 0) {
            throw new Error("Invalid GitLab merge request fetch request.");
          }
          const mergeRequestIid = mrIid;
          if (remote.startsWith("-")) {
            throw new Error('GitLab merge request fetch remote must not start with "-".');
          }
          try {
            const { stdout } = await this.git(["remote"], worktreePath);
            const remotes = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
            if (!remotes.includes(remote)) {
              throw new Error(`Remote "${remote}" is not configured.`);
            }
            await this.git(
              ["fetch", "--no-tags", remote, `refs/merge-requests/${mergeRequestIid}/head`],
              worktreePath
            );
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "fetch"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async push(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        void params.publish;
        try {
          try {
            const target = await resolveRelayPushTarget(
              this.git.bind(this),
              worktreePath,
              params.pushTarget
            );
            const args = [
              "push",
              ...params.forceWithLease === true ? ["--force-with-lease"] : [],
              "--set-upstream",
              ...target ? [target.remote, target.refspec] : ["origin", "HEAD"]
            ];
            await this.git(args, worktreePath);
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "push"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async pullWithArgs(params, pullArgs) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        try {
          try {
            if (params.pushTarget !== void 0) {
              assertGitPushTargetShape(params.pushTarget);
              const pushTarget = params.pushTarget;
              await this.git(["check-ref-format", "--branch", pushTarget.branchName], worktreePath);
              await this.git(
                ["pull", ...pullArgs, pushTarget.remoteName, pushTarget.branchName],
                worktreePath
              );
              return;
            }
            const upstream = await resolveEffectiveGitUpstream((args) => this.git(args, worktreePath));
            if (upstream && !upstream.isConfiguredUpstream) {
              await this.git(
                ["pull", ...pullArgs, upstream.remoteName, upstream.branchName],
                worktreePath
              );
              return;
            }
            await this.git(["pull", ...pullArgs], worktreePath);
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "pull"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async pull(params) {
        await this.pullWithArgs(params, []);
      }
      async fastForward(params) {
        await this.pullWithArgs(params, ["--ff-only"]);
      }
      async rebaseFromBase(params) {
        this.clearGitMutationReadCaches();
        const worktreePath = params.worktreePath;
        const baseRef = params.baseRef;
        try {
          try {
            const source = await resolveGitRemoteRebaseSource(
              ((args) => this.git(args, worktreePath)),
              baseRef
            );
            await this.git(["pull", "--rebase", source.remoteName, source.branchName], worktreePath);
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error, "pull"));
          }
        } finally {
          this.clearGitMutationReadCaches();
        }
      }
      async branchDiff(params, context) {
        const worktreePath = params.worktreePath;
        const baseRef = params.baseRef;
        if (baseRef.startsWith("-")) {
          throw new Error('Base ref must not start with "-"');
        }
        const options = {
          includePatch: params.includePatch,
          filePath: params.filePath,
          oldPath: params.oldPath
        };
        const result = await this.gitDiffReadDedupe.run(
          stableInFlightKey([
            "branchDiff",
            worktreePath,
            baseRef,
            options.includePatch ?? null,
            options.filePath ?? null,
            options.oldPath ?? null
          ]),
          () => branchDiffEntries(
            this.git.bind(this),
            this.gitBuffer.bind(this),
            worktreePath,
            baseRef,
            options
          )
        );
        return this.maybeStreamResponse(result, params, context);
      }
      async commitDiff(params, context) {
        const worktreePath = params.worktreePath;
        const args = {
          commitOid: params.commitOid,
          parentOid: params.parentOid,
          filePath: params.filePath,
          oldPath: params.oldPath
        };
        const result = await this.gitDiffReadDedupe.run(
          stableInFlightKey([
            "commitDiff",
            worktreePath,
            args.commitOid,
            args.parentOid ?? null,
            args.filePath,
            args.oldPath ?? null
          ]),
          () => commitDiffEntry(this.gitBuffer.bind(this), worktreePath, args)
        );
        return this.maybeStreamResponse(result, params, context);
      }
      async exec(params, context) {
        const args = params.args;
        const cwd = params.cwd;
        validateGitExecArgs(args);
        const run = () => this.git(args, cwd, { signal: context?.signal });
        const { stdout, stderr } = gitExecMutatesRepository(args) ? await this.runWithGitReadCacheClear(run) : await run();
        return this.maybeStreamResponse({ stdout, stderr }, params, context);
      }
      async clone(params, context) {
        const args = params.args;
        const cwd = params.cwd;
        const progressId = params.progressId;
        validateGitExecArgs(args);
        if (typeof progressId !== "string" || progressId.length === 0) {
          throw new Error("Missing clone progress id.");
        }
        if (args[0] !== "clone") {
          throw new Error("git.clone only supports clone commands.");
        }
        return await this.runWithGitReadCacheClear(
          () => this.spawnClone(args, cwd, progressId, context)
        );
      }
      async spawnClone(args, cwd, progressId, context) {
        return await new Promise((resolve8, reject) => {
          const child = (0, import_node_child_process2.spawn)("git", args, {
            cwd: expandTilde(cwd),
            env: buildRelayGitEnv(),
            stdio: ["ignore", "pipe", "pipe"]
          });
          let stdout = "";
          let stderr = "";
          let settled = false;
          const cleanup = () => {
            context?.signal?.removeEventListener("abort", onAbort);
          };
          const onAbort = () => {
            child.kill();
          };
          context?.signal?.addEventListener("abort", onAbort, { once: true });
          child.stdout?.on("data", (chunk) => {
            stdout = (stdout + chunk.toString("utf-8")).slice(-4096);
          });
          child.stderr?.on("data", (chunk) => {
            const text = chunk.toString("utf-8");
            stderr = (stderr + text).slice(-4096);
            for (const line of text.split(/[\r\n]+/)) {
              const match = line.match(/^([\w\s]+):\s+(\d+)%/);
              if (match) {
                this.dispatcher.notify("git.cloneProgress", {
                  progressId,
                  phase: match[1].trim(),
                  percent: Number.parseInt(match[2], 10)
                });
              }
            }
          });
          child.on("error", (error) => {
            if (settled) {
              return;
            }
            settled = true;
            cleanup();
            reject(error);
          });
          child.on("close", (code, signal) => {
            if (settled) {
              return;
            }
            settled = true;
            cleanup();
            if (context?.signal?.aborted) {
              reject(new Error("Clone aborted"));
              return;
            }
            if (code === 0 && !signal) {
              resolve8({ stdout, stderr });
              return;
            }
            reject(new Error(`Clone failed: ${getGitCloneFailureMessage(stderr)}`));
          });
        });
      }
      async renameCurrentBranch(params) {
        return this.runWithGitReadCacheClear(async () => {
          const worktreePath = params.worktreePath;
          const newBranch = params.newBranch;
          if (typeof worktreePath !== "string" || typeof newBranch !== "string") {
            throw new Error("Invalid branch rename request.");
          }
          if (newBranch.startsWith("-")) {
            throw new Error('Branch name must not start with "-".');
          }
          try {
            await this.git(["check-ref-format", "--branch", newBranch], worktreePath);
            await this.git(["branch", "-m", newBranch], worktreePath);
          } catch (error) {
            throw new Error(normalizeGitErrorMessage(error));
          }
        });
      }
      async forceDeletePreservedBranch(params) {
        const repoPath = params.repoPath;
        const branchName = params.branchName;
        const expectedHead = params.expectedHead;
        if (typeof repoPath !== "string" || typeof branchName !== "string" || typeof expectedHead !== "string") {
          throw new Error("Invalid preserved branch force-delete request.");
        }
        if (!repoPath || repoPath.includes("\0") || expectedHead.includes("\0")) {
          throw new Error("Invalid preserved branch force-delete request.");
        }
        return this.runWithGitReadCacheClear(
          () => forceDeletePreservedRelayBranch(this.git.bind(this), repoPath, branchName, expectedHead)
        );
      }
      async isGitRepo(params) {
        const dirPath = params.dirPath;
        try {
          const { stdout } = await this.git(["rev-parse", "--show-toplevel"], dirPath);
          return { isRepo: true, rootPath: stdout.trim() };
        } catch {
          return { isRepo: false, rootPath: null };
        }
      }
      async readRepoLocation(repoPath) {
        try {
          return await this.gitCapabilities.runWithFallback(
            "rev-parse-path-format",
            async () => {
              const { stdout } = await this.git(
                ["rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir"],
                repoPath
              );
              if (hasUnsupportedRevParsePathFormatEcho(stdout)) {
                this.gitCapabilities.rememberUnsupported("rev-parse-path-format");
              }
              return parseRelayRepoLocation(repoPath, stdout);
            },
            async () => {
              const { stdout } = await this.git(
                ["rev-parse", "--show-toplevel", "--git-common-dir"],
                repoPath
              );
              return parseRelayRepoLocation(repoPath, stdout);
            },
            isUnsupportedRevParsePathFormatError
          );
        } catch {
          return void 0;
        }
      }
      async normalizeMainWorktreePath(repoPath, worktrees) {
        const mainIndex = worktrees.findIndex((worktree) => worktree.isMainWorktree === true);
        const mainWorktree = worktrees[mainIndex];
        const mainPath = typeof mainWorktree?.path === "string" ? mainWorktree.path : "";
        const resolvedRepoPath = expandTilde(repoPath);
        if (!mainPath || areRelayWorktreePathsEqual2(mainPath, resolvedRepoPath)) {
          return worktrees;
        }
        const location = await this.readRepoLocation(resolvedRepoPath);
        if (!location) {
          return worktrees;
        }
        if (!areRelayWorktreePathsEqual2(mainPath, location.commonDir)) {
          return worktrees;
        }
        const normalized = [...worktrees];
        normalized[mainIndex] = { ...mainWorktree, path: location.topLevel };
        return normalized;
      }
      async listWorktrees(params, context) {
        const repoPath = params.repoPath;
        return this.gitCapabilities.runWithFallback(
          "worktree-list-z",
          async () => {
            const { stdout } = await this.git(["worktree", "list", "--porcelain", "-z"], repoPath, {
              signal: context?.signal
            });
            return this.normalizeMainWorktreePath(
              repoPath,
              parseWorktreeList(stdout, { nulDelimited: true })
            );
          },
          async () => {
            try {
              const { stdout } = await this.git(["worktree", "list", "--porcelain"], repoPath, {
                signal: context?.signal
              });
              return this.normalizeMainWorktreePath(repoPath, parseWorktreeList(stdout));
            } catch {
              return [];
            }
          },
          isUnsupportedWorktreeListZError
        ).catch(() => []);
      }
      async addWorktree(params) {
        return this.runWithGitReadCacheClear(() => addWorktreeOp(this.git.bind(this), params));
      }
      async removeWorktree(params) {
        return this.runWithGitReadCacheClear(
          () => removeWorktreeOp(this.git.bind(this), params, this.gitCapabilities)
        );
      }
      async worktreeIsClean(params) {
        return worktreeIsCleanOp(this.git.bind(this), params);
      }
      async refreshLocalBaseRefForWorktreeCreate(params) {
        return this.runWithGitReadCacheClear(
          () => refreshLocalBaseRefForWorktreeCreateOp(this.git.bind(this), params, this.gitCapabilities)
        );
      }
    };
  }
});

// src/relay/agent-git-handler.ts
var agent_git_handler_exports = {};
__export(agent_git_handler_exports, {
  GitValidationError: () => GitValidationError,
  handleGitExec: () => handleGitExec,
  handleGitExecStream: () => handleGitExecStream,
  handleGitPrCreate: () => handleGitPrCreate,
  handleGitWorktreeAdd: () => handleGitWorktreeAdd,
  handleGitWorktreeList: () => handleGitWorktreeList,
  handleGitWorktreeRemove: () => handleGitWorktreeRemove,
  validateGitArgs: () => validateGitArgs
});
function resumeFrom(params) {
  const t = params["_trace"];
  if (t && typeof t === "object" && typeof t.id === "string") {
    return { id: t.id };
  }
  return void 0;
}
function validateGitArgs(args) {
  if (args.length === 0) {
    throw new GitValidationError("GIT_NO_SUBCOMMAND", "git args must not be empty \u2014 provide a subcommand");
  }
  if (!ALLOWED_GIT_SUBCOMMANDS2.has(args[0])) {
    throw new GitValidationError(
      "GIT_DISALLOWED_SUBCOMMAND",
      `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS2].sort().join(", ")}`
    );
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new GitValidationError(
        "GIT_SHELL_METACHARACTER_IN_ARG",
        `Unsafe character in git argument: "${arg}"`
      );
    }
  }
}
async function handleGitExec(id, params, config, log) {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : [];
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const timeout = Math.min(typeof params.timeout === "number" ? params.timeout : 3e4, 6e4);
  const argsStr = rawArgs.join(" ").slice(0, 80);
  const span = gitTracer.start({ method: "git.exec", cmd: argsStr, cwd }, resumeFrom(params));
  try {
    validateGitArgs(rawArgs);
  } catch (err) {
    if (err instanceof GitValidationError) {
      span.fail(`validation: ${err.message}`, { cmd: argsStr });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: err.message } };
    }
    throw err;
  }
  return new Promise((resolve8) => {
    const child = (0, import_node_child_process3.spawn)("git", rawArgs, {
      cwd,
      env: config.toolEnv,
      stdio: ["pipe", "pipe", "pipe"],
      shell: false
      // mandatory: no shell injection
    });
    const stdout = [];
    const stderr = [];
    const timer = setTimeout(() => {
      child.kill("SIGTERM");
      span.fail(`timeout after ${timeout}ms`, { cmd: argsStr });
      resolve8({
        jsonrpc: "2.0",
        id,
        error: { code: AgentErrorCode.ServerError, message: `git.exec timeout after ${timeout}ms` }
      });
    }, timeout);
    child.stdout?.on("data", (chunk) => stdout.push(chunk.toString()));
    child.stderr?.on("data", (chunk) => stderr.push(chunk.toString()));
    child.on("close", (code) => {
      clearTimeout(timer);
      const exitCode = code ?? 0;
      log.info(`git.exec: ${rawArgs.join(" ")} \u2192 exitCode=${exitCode}`);
      const outLen = stdout.join("").length;
      if (exitCode === 0) {
        span.ok({ cmd: argsStr, exitCode, outLen });
      } else {
        span.fail(`git exit ${exitCode}`, { cmd: argsStr, exitCode, outLen });
      }
      resolve8({
        jsonrpc: "2.0",
        id,
        result: { stdout: stdout.join(""), stderr: stderr.join(""), exitCode }
      });
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      span.fail(err, { cmd: argsStr });
      resolve8({ jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: err.message } });
    });
    child.stdin?.end();
  });
}
async function handleGitExecStream(ws, wireState, id, params, config, log) {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : [];
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  try {
    validateGitArgs(rawArgs);
  } catch (err) {
    if (err instanceof GitValidationError) {
      sendFrame(ws, wireState, { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: err.message } });
      return;
    }
    throw err;
  }
  const child = (0, import_node_child_process3.spawn)("git", rawArgs, {
    cwd,
    env: config.toolEnv,
    stdio: ["pipe", "pipe", "pipe"],
    shell: false
  });
  function sendChunk(line, source) {
    sendFrame(ws, wireState, {
      jsonrpc: "2.0",
      id,
      result: { type: "stream.chunk", line, ...source ? { source } : {} }
    });
  }
  function sendEnd(exitCode) {
    sendFrame(ws, wireState, { jsonrpc: "2.0", id, result: { type: "stream.end", exitCode } });
  }
  child.stdout?.on("data", (chunk) => {
    chunk.toString("utf8").split("\n").filter((l) => l.length > 0).forEach((l) => sendChunk(l));
  });
  child.stderr?.on("data", (chunk) => {
    chunk.toString("utf8").split("\n").filter((l) => l.length > 0).forEach((l) => sendChunk(l, "stderr"));
  });
  child.on("close", (code) => {
    const exitCode = code ?? 0;
    log.info(`git.execStream: ${rawArgs.join(" ")} \u2192 exitCode=${exitCode}`);
    sendEnd(exitCode);
  });
  child.on("error", (err) => {
    sendFrame(ws, wireState, {
      jsonrpc: "2.0",
      id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    });
  });
  child.stdin?.end();
}
function sendFrame(ws, wireState, payload) {
  if (ws.readyState === 1) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)));
  }
}
async function handleGitPrCreate(id, params, config, log) {
  const title = typeof params.title === "string" ? params.title.trim() : "";
  const body = typeof params.body === "string" ? params.body : "";
  const base = typeof params.base === "string" ? params.base.trim() : "main";
  const draft = params.draft === true;
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const span = gitTracer.start({ method: "git.pr.create", title: title.slice(0, 40), base });
  if (!title) {
    span.fail("missing title", { method: "git.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: title" } };
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    span.fail("unsafe characters in params", { method: "git.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in PR params" } };
  }
  const ghArgs = ["pr", "create", "--title", title, "--body", body, "--base", base];
  if (draft) {ghArgs.push("--draft");}
  const { homedir: homedir6 } = await import("node:os");
  const env = {
    ...config.toolEnv,
    ...userId ? { GH_CONFIG_DIR: `${homedir6()}/.config/gh/${userId}/` } : {},
    GH_NO_UPDATE_NOTIFIER: "1",
    GH_PROMPT_DISABLED: "1"
  };
  span.step("ghExec", { base });
  try {
    const { stdout, stderr } = await execFileAsync2("gh", ghArgs, { cwd, env, timeout: 3e4 });
    const url = stdout.trim();
    log.info(`git.pr.create: PR created \u2192 ${url}`);
    span.ok({ url });
    return { jsonrpc: "2.0", id, result: { url, stdout, stderr } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    log.error(`git.pr.create failed: ${msg}`);
    span.fail(err, { method: "git.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitWorktreeList(id, params, config, log) {
  const cwd = typeof params.cwd === "string" ? params.cwd : config.workDir;
  try {
    const { execFile: execFile4 } = await import("node:child_process");
    const { promisify: promisify3 } = await import("node:util");
    const { parseWorktreePorcelain: parseWorktreePorcelain2 } = await Promise.resolve().then(() => (init_git_handler(), git_handler_exports));
    const execAsync = promisify3(execFile4);
    const { stdout } = await execAsync("git", ["worktree", "list", "--porcelain"], { cwd, timeout: 1e4 });
    const worktrees = parseWorktreePorcelain2(stdout);
    log.info(`git.worktree.list: cwd=${cwd} count=${worktrees.length}`);
    return { jsonrpc: "2.0", id, result: { worktrees } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: `git.worktree.list failed: ${msg}` } };
  }
}
async function handleGitWorktreeAdd(id, params, config, log) {
  const worktreePath = typeof params.path === "string" ? params.path.trim() : "";
  const branch = typeof params.branch === "string" ? params.branch.trim() : "";
  const createBranch = params.createBranch === true;
  const cwd = typeof params.cwd === "string" ? params.cwd : config.workDir;
  const span = Tracers.worktreeCreate.start({ path: worktreePath, branch, cwd }, resumeFrom(params));
  if (!worktreePath || !branch) {
    span.fail("missing required params", { path: worktreePath, branch });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required params: path, branch" } };
  }
  if (SHELL_METACHARACTERS.test(worktreePath) || SHELL_METACHARACTERS.test(branch)) {
    span.fail("unsafe characters in params", { path: worktreePath, branch });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in worktree params" } };
  }
  try {
    const { validateWorktreePath: validateWorktreePath2 } = await Promise.resolve().then(() => (init_git_handler(), git_handler_exports));
    span.step("validate-path", { path: worktreePath });
    validateWorktreePath2(["worktree", "add", worktreePath], cwd);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(msg, { path: worktreePath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: msg } };
  }
  const args = createBranch ? ["worktree", "add", "-b", branch, worktreePath] : ["worktree", "add", worktreePath, branch];
  span.step("git-worktree-add-exec", { branch });
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15e3, _trace: { id: span.id } },
    config,
    log
  );
  if (result && typeof result === "object" && "error" in result) {
    span.fail(result.error.message, { path: worktreePath });
  } else {
    span.ok({ path: worktreePath, branch });
  }
  return result;
}
async function handleGitWorktreeRemove(id, params, config, log) {
  const path12 = typeof params.path === "string" ? params.path.trim() : "";
  const force = params.force === true;
  const span = Tracers.worktreeDelete.start({ path: path12, force }, resumeFrom(params));
  if (!path12) {
    span.fail("missing required param: path");
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  if (SHELL_METACHARACTERS.test(path12)) {
    span.fail("unsafe characters in path", { path: path12 });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in path" } };
  }
  const args = ["worktree", "remove", path12];
  if (force) {args.push("--force");}
  span.step("git-worktree-remove-exec", { force });
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15e3, _trace: { id: span.id } },
    config,
    log
  );
  if (result && typeof result === "object" && "error" in result) {
    span.fail(result.error.message, { path: path12 });
  } else {
    span.ok({ path: path12, force });
  }
  return result;
}
var import_node_child_process3, import_node_util2, execFileAsync2, gitTracer, ALLOWED_GIT_SUBCOMMANDS2, SHELL_METACHARACTERS, GitValidationError;
var init_agent_git_handler = __esm({
  "src/relay/agent-git-handler.ts"() {
    "use strict";
    import_node_child_process3 = require("node:child_process");
    import_node_util2 = require("node:util");
    init_agent_wire();
    init_agent_wire_protocol();
    init_trace();
    init_tracers();
    execFileAsync2 = (0, import_node_util2.promisify)(import_node_child_process3.execFile);
    gitTracer = createTracer("agent:git");
    ALLOWED_GIT_SUBCOMMANDS2 = /* @__PURE__ */ new Set([
      "status",
      "diff",
      "add",
      "restore",
      "commit",
      "push",
      "pull",
      "fetch",
      "branch",
      "checkout",
      "merge",
      "rebase",
      "stash",
      "log",
      "worktree",
      "remote",
      "tag",
      "show",
      "rev-parse",
      "config",
      "describe",
      "shortlog"
    ]);
    SHELL_METACHARACTERS = /[&|;$`<>\\!]/;
    GitValidationError = class extends Error {
      constructor(code, message) {
        super(message);
        this.code = code;
        this.name = "GitValidationError";
      }
      code;
    };
  }
});

// src/shared/search-match-count.ts
var init_search_match_count = __esm({
  "src/shared/search-match-count.ts"() {
    "use strict";
  }
});

// src/shared/string-utils.ts
var init_string_utils = __esm({
  "src/shared/string-utils.ts"() {
    "use strict";
  }
});

// src/shared/text-search.ts
var SEARCH_MAX_FILE_SIZE;
var init_text_search = __esm({
  "src/shared/text-search.ts"() {
    "use strict";
    init_search_match_count();
    init_string_utils();
    SEARCH_MAX_FILE_SIZE = 5 * 1024 * 1024;
  }
});

// src/shared/image-file-extensions.ts
var IMAGE_FILE_MIME_TYPES, IMAGE_FILE_EXTENSIONS;
var init_image_file_extensions = __esm({
  "src/shared/image-file-extensions.ts"() {
    "use strict";
    IMAGE_FILE_MIME_TYPES = {
      ".png": "image/png",
      ".jpg": "image/jpeg",
      ".jpeg": "image/jpeg",
      ".gif": "image/gif",
      ".svg": "image/svg+xml",
      ".webp": "image/webp",
      ".bmp": "image/bmp",
      ".ico": "image/x-icon"
    };
    IMAGE_FILE_EXTENSIONS = Object.freeze(Object.keys(IMAGE_FILE_MIME_TYPES));
  }
});

// src/shared/file-listing-cancellation.ts
var init_file_listing_cancellation = __esm({
  "src/shared/file-listing-cancellation.ts"() {
    "use strict";
  }
});

// src/shared/quick-open-filter.ts
var init_quick_open_filter = __esm({
  "src/shared/quick-open-filter.ts"() {
    "use strict";
  }
});

// src/relay/fs-handler-list-files.ts
var init_fs_handler_list_files = __esm({
  "src/relay/fs-handler-list-files.ts"() {
    "use strict";
    init_file_listing_cancellation();
    init_quick_open_filter();
  }
});

// src/relay/fs-handler-utils.ts
function isBinaryBuffer2(buffer) {
  const len = Math.min(buffer.length, 8192);
  for (let i = 0; i < len; i++) {
    if (buffer[i] === 0) {
      return true;
    }
  }
  return false;
}
async function isBinaryFilePrefix(filePath) {
  const handle = await (0, import_promises6.open)(filePath, "r");
  try {
    const probe = Buffer.alloc(BINARY_PROBE_BYTES);
    const { bytesRead } = await handle.read(probe, 0, probe.length, 0);
    return isBinaryBuffer2(probe.subarray(0, bytesRead));
  } finally {
    await handle.close();
  }
}
function checkRgAvailable() {
  return new Promise((resolve8) => {
    let settled = false;
    const child = (0, import_node_child_process4.execFile)("rg", ["--version"]);
    let timeout = null;
    const cleanup = () => {
      if (timeout) {
        clearTimeout(timeout);
        timeout = null;
      }
      child.off("error", onError);
      child.off("close", onClose);
    };
    const settle = (available, options) => {
      if (settled) {
        return;
      }
      settled = true;
      cleanup();
      if (options?.kill) {
        child.kill();
      }
      resolve8(available);
    };
    const onError = () => settle(false);
    const onClose = (code) => settle(code === 0);
    child.once("error", onError);
    child.once("close", onClose);
    timeout = setTimeout(() => settle(false, { kill: true }), RG_AVAILABILITY_TIMEOUT_MS);
    if (typeof timeout.unref === "function") {
      timeout.unref();
    }
  });
}
var import_node_child_process4, import_promises6, MAX_TEXT_FILE_SIZE, MAX_PREVIEWABLE_BINARY_SIZE, BINARY_PROBE_BYTES, IMAGE_MIME_TYPES, RG_AVAILABILITY_TIMEOUT_MS;
var init_fs_handler_utils = __esm({
  "src/relay/fs-handler-utils.ts"() {
    "use strict";
    import_node_child_process4 = require("node:child_process");
    import_promises6 = require("node:fs/promises");
    init_text_search();
    init_image_file_extensions();
    init_fs_handler_list_files();
    MAX_TEXT_FILE_SIZE = 10 * 1024 * 1024;
    MAX_PREVIEWABLE_BINARY_SIZE = 50 * 1024 * 1024;
    BINARY_PROBE_BYTES = 8192;
    IMAGE_MIME_TYPES = {
      ...IMAGE_FILE_MIME_TYPES,
      ".pdf": "application/pdf"
    };
    RG_AVAILABILITY_TIMEOUT_MS = 5e3;
  }
});

// src/relay/fs-handler-file-read.ts
async function readRelayFileContent(filePath) {
  const stats = await (0, import_promises7.stat)(filePath);
  const mimeType = IMAGE_MIME_TYPES[(0, import_node_path5.extname)(filePath).toLowerCase()];
  const sizeLimit = mimeType ? MAX_PREVIEWABLE_BINARY_SIZE : MAX_TEXT_FILE_SIZE;
  if (stats.size > sizeLimit) {
    throw new Error(
      `File too large: ${(stats.size / 1024 / 1024).toFixed(1)}MB exceeds ${sizeLimit / 1024 / 1024}MB limit`
    );
  }
  if (mimeType) {
    const buffer2 = await (0, import_promises7.readFile)(filePath);
    return { content: buffer2.toString("base64"), isBinary: true, isImage: true, mimeType };
  }
  if (stats.size > BINARY_PROBE_BYTES && await isBinaryFilePrefix(filePath)) {
    return { content: "", isBinary: true };
  }
  const buffer = await (0, import_promises7.readFile)(filePath);
  if (isBinaryBuffer2(buffer)) {
    return { content: "", isBinary: true };
  }
  return { content: buffer.toString("utf-8"), isBinary: false };
}
var import_promises7, import_node_path5;
var init_fs_handler_file_read = __esm({
  "src/relay/fs-handler-file-read.ts"() {
    "use strict";
    import_promises7 = require("node:fs/promises");
    import_node_path5 = require("node:path");
    init_protocol();
    init_fs_handler_utils();
  }
});

// src/relay/fs-agent-extensions.ts
var fs_agent_extensions_exports = {};
__export(fs_agent_extensions_exports, {
  cleanupAgentWatches: () => cleanupAgentWatches,
  handleFsGlob: () => handleFsGlob,
  handleFsGrep: () => handleFsGrep,
  handleFsMkdir: () => handleFsMkdir,
  handleFsReadDir: () => handleFsReadDir,
  handleFsReadFile: () => handleFsReadFile,
  handleFsRmdir: () => handleFsRmdir,
  handleFsStat: () => handleFsStat,
  handleFsUnwatch: () => handleFsUnwatch,
  handleFsWatch: () => handleFsWatch,
  handleFsWriteFile: () => handleFsWriteFile,
  handlePreflightCheck: () => handlePreflightCheck,
  handleShellEval: () => handleShellEval
});
async function handleFsReadDir(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const depth = typeof params.depth === "number" ? Math.min(params.depth, 5) : 1;
  const span = fsTracer.start({ method: "fs.readDir", path: rawPath || "(empty)", depth });
  if (!rawPath) {
    span.fail("missing param: path", { method: "fs.readDir" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  try {
    const st = await (0, import_promises8.stat)(absPath);
    if (!st.isDirectory()) {
      span.fail("not a directory", { path: absPath });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: `Not a directory: ${absPath}` } };
    }
    const entries = await readDirRecursive(absPath, depth, 1);
    span.ok({ path: absPath, entries: entries.length });
    return { jsonrpc: "2.0", id, result: { entries, path: absPath } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { path: absPath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function readDirRecursive(dir, maxDepth, currentDepth) {
  const entries = await (0, import_promises8.readdir)(dir, { withFileTypes: true });
  const nodes = await Promise.all(
    entries.map(async (entry) => {
      const fullPath = (0, import_node_path6.join)(dir, entry.name);
      const node = {
        path: fullPath,
        name: entry.name,
        type: entry.isDirectory() ? "directory" : "file",
        size: entry.isFile() ? (await (0, import_promises8.stat)(fullPath)).size : void 0
      };
      if (entry.isDirectory() && currentDepth < maxDepth) {
        node.children = await readDirRecursive(fullPath, maxDepth, currentDepth + 1);
      }
      return node;
    })
  );
  return nodes.sort((a, b) => {
    if (a.type !== b.type) {return a.type === "directory" ? -1 : 1;}
    return a.name.localeCompare(b.name);
  });
}
async function handleFsReadFile(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const span = fsTracer.start({ method: "fs.readFile", path: rawPath || "(empty)" });
  if (!rawPath) {
    span.fail("missing param: path", { method: "fs.readFile" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  try {
    const result = await readRelayFileContent(absPath);
    span.ok({ path: absPath, bytes: result.content.length, binary: result.isBinary });
    return {
      jsonrpc: "2.0",
      id,
      result: {
        content: result.content,
        encoding: result.isBinary ? "base64" : "utf-8",
        isBinary: result.isBinary,
        path: absPath
      }
    };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("File too large") || msg.includes("MAX_TEXT_FILE_SIZE")) {
      span.fail("file too large", { path: absPath });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "FILE_TOO_LARGE" } };
    }
    if (msg.includes("ENOENT") || msg.includes("not found")) {
      span.fail("file not found", { path: absPath });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.PathNotFound, message: `File not found: ${absPath}` } };
    }
    span.fail(err, { path: absPath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleFsGrep(id, params, config) {
  const root = typeof params.root === "string" && params.root ? params.root : config.workDir;
  const pattern = typeof params.pattern === "string" ? params.pattern : "";
  const maxResults = typeof params.maxResults === "number" ? Math.min(params.maxResults, 200) : 50;
  const span = fsTracer.start({ method: "fs.grep", pattern: pattern || "(empty)", root });
  if (!pattern) {
    span.fail("missing param: pattern", { method: "fs.grep" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: pattern" } };
  }
  const absRoot = (0, import_node_path6.isAbsolute)(root) ? root : (0, import_node_path6.join)(config.workDir, root);
  const rgAvailable = await checkRgAvailable();
  try {
    const matches = rgAvailable ? await grepWithRg(pattern, absRoot, maxResults, config) : await grepFallback(pattern, absRoot, maxResults, config);
    span.ok({ pattern, matches: matches.length, truncated: matches.length >= maxResults, tool: rgAvailable ? "rg" : "grep" });
    return {
      jsonrpc: "2.0",
      id,
      result: { matches, total: matches.length, truncated: matches.length >= maxResults }
    };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { pattern, root: absRoot });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
function grepWithRg(pattern, root, maxResults, config) {
  return new Promise((resolve8, reject) => {
    const child = (0, import_node_child_process5.spawn)(
      "rg",
      ["--json", "--ignore-case", "--max-count", String(maxResults), pattern, root],
      { env: config.toolEnv, stdio: ["pipe", "pipe", "pipe"], shell: false }
    );
    let output = "";
    child.stdout?.on("data", (c) => {
      output += c.toString();
    });
    child.on("close", (code) => {
      if (code !== 0 && code !== 1) {
        reject(new Error(`rg exited with code ${code}`));
        return;
      }
      const matches = [];
      for (const line of output.split("\n")) {
        if (!line.trim()) {continue;}
        try {
          const obj = JSON.parse(line);
          if (obj.type === "match") {
            matches.push({
              file: obj.data.path.text,
              line: obj.data.line_number,
              text: obj.data.lines.text.trimEnd()
            });
            if (matches.length >= maxResults) {break;}
          }
        } catch {
        }
      }
      resolve8(matches);
    });
    child.on("error", reject);
    child.stdin?.end();
  });
}
function grepFallback(pattern, root, maxResults, config) {
  return new Promise((resolve8, reject) => {
    const child = (0, import_node_child_process5.spawn)(
      "grep",
      [
        "-r",
        "-n",
        "-i",
        "--include=*.ts",
        "--include=*.tsx",
        "--include=*.js",
        "--include=*.go",
        "--include=*.py",
        "--include=*.md",
        pattern,
        root
      ],
      { env: config.toolEnv, stdio: ["pipe", "pipe", "pipe"], shell: false }
    );
    let output = "";
    child.stdout?.on("data", (c) => {
      output += c.toString();
    });
    child.on("close", (code) => {
      if (code !== 0 && code !== 1) {
        reject(new Error(`grep exited with code ${code}`));
        return;
      }
      const matches = [];
      for (const raw of output.split("\n")) {
        const m = raw.match(/^(.+?):(\d+):(.*)$/);
        if (m) {
          matches.push({ file: m[1], line: Number.parseInt(m[2], 10), text: m[3] });
          if (matches.length >= maxResults) {break;}
        }
      }
      resolve8(matches);
    });
    child.on("error", reject);
    child.stdin?.end();
  });
}
async function handlePreflightCheck(id, params, config) {
  const services = Array.isArray(params.services) ? params.services.map(String) : [];
  const results = {};
  const span = preflightTracer.start({ services: services.join(",") || "(empty)" });
  await Promise.all(services.map(async (service) => {
    try {
      switch (service) {
        case "github-cli":
          results[service] = await checkBinaryAvailable("gh", config);
          break;
        case "ripgrep":
          results[service] = await checkRgAvailable();
          break;
        case "docker":
          results[service] = await checkBinaryAvailable("docker", config);
          break;
        case "claude":
          results[service] = await checkBinaryAvailable("claude", config);
          break;
        default:
          results[service] = false;
      }
    } catch {
      results[service] = false;
    }
  }));
  const failedServices = Object.entries(results).filter(([, ok]) => !ok).map(([svc]) => svc);
  if (failedServices.length > 0) {
    span.fail(`unavailable: ${failedServices.join(",")}`, { failedCount: failedServices.length });
  } else {
    span.ok({ checkedCount: services.length });
  }
  return { jsonrpc: "2.0", id, result: results };
}
function checkBinaryAvailable(binary, config) {
  return new Promise((resolve8) => {
    const child = (0, import_node_child_process5.spawn)(binary, ["--version"], {
      env: config.toolEnv,
      stdio: ["pipe", "pipe", "pipe"],
      shell: false
    });
    const timer = setTimeout(() => {
      child.kill();
      resolve8(false);
    }, 5e3);
    child.on("close", (code) => {
      clearTimeout(timer);
      resolve8(code === 0);
    });
    child.on("error", () => {
      clearTimeout(timer);
      resolve8(false);
    });
    child.stdin?.end();
  });
}
async function handleFsStat(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  if (!rawPath) {
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  try {
    const st = await (0, import_promises8.stat)(absPath);
    return {
      jsonrpc: "2.0",
      id,
      result: {
        path: absPath,
        size: st.size,
        mtime: st.mtime.toISOString(),
        isDir: st.isDirectory(),
        isFile: st.isFile(),
        isLink: st.isSymbolicLink(),
        mode: st.mode.toString(8)
      }
    };
  } catch (err) {
    const nodeErr = err;
    if (nodeErr.code === "ENOENT") {
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: `Not found: ${absPath}` } };
    }
    const msg = err instanceof Error ? err.message : String(err);
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleFsGlob(id, params, config) {
  const pattern = typeof params.pattern === "string" ? params.pattern : "";
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const ignore = Array.isArray(params.ignore) ? params.ignore.map(String) : ["node_modules", ".git", "dist", "out"];
  const MAX_RESULTS = 200;
  if (!pattern) {
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: pattern" } };
  }
  const filePattern = pattern.split("/").pop() ?? pattern;
  const ignoreArgs = ignore.flatMap((p) => [
    "-not",
    "-path",
    `*/${p}/*`,
    "-not",
    "-name",
    p
  ]);
  const results = await new Promise((resolve8, reject) => {
    const child = (0, import_node_child_process5.spawn)("find", [
      cwd,
      "-maxdepth",
      "10",
      ...ignoreArgs,
      "-name",
      filePattern,
      "-type",
      "f"
    ], { shell: false });
    const lines = [];
    child.stdout?.on("data", (chunk) => {
      const newLines = chunk.toString().split("\n").filter((l) => l.trim());
      lines.push(...newLines);
      if (lines.length > MAX_RESULTS) {child.kill("SIGTERM");}
    });
    child.on("close", () => resolve8(lines.slice(0, MAX_RESULTS)));
    child.on("error", reject);
  });
  const relativePaths = results.map(
    (p) => p.startsWith(`${cwd  }/`) ? p.slice(cwd.length + 1) : p
  );
  return {
    jsonrpc: "2.0",
    id,
    result: { paths: relativePaths, cwd, total: relativePaths.length }
  };
}
async function handleFsWriteFile(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const content = typeof params.content === "string" ? params.content : "";
  const encoding = typeof params.encoding === "string" ? params.encoding : "utf-8";
  if (!rawPath) {
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  const resolvedPath = (0, import_node_path6.resolve)(absPath);
  const resolvedWork = (0, import_node_path6.resolve)(config.workDir);
  const span = fsTracer.start({ method: "fs.writeFile", path: rawPath });
  if (!resolvedPath.startsWith(`${resolvedWork  }/`) && resolvedPath !== resolvedWork) {
    span.fail("path outside project root", { path: rawPath });
    return {
      jsonrpc: "2.0",
      id,
      error: { code: AgentErrorCode.InvalidParams, message: `Path outside project root: ${rawPath}` }
    };
  }
  const MAX_WRITE = 10 * 1024 * 1024;
  const byteLen = Buffer.byteLength(content, encoding);
  if (byteLen > MAX_WRITE) {
    span.fail("content too large", { path: rawPath, bytes: byteLen });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Content too large: max 10MB" } };
  }
  try {
    await (0, import_promises8.mkdir)((0, import_node_path6.dirname)(resolvedPath), { recursive: true });
    await (0, import_promises8.writeFile)(resolvedPath, content, { encoding });
    span.ok({ path: resolvedPath, bytes: byteLen });
    return {
      jsonrpc: "2.0",
      id,
      result: { ok: true, path: resolvedPath, bytes: byteLen }
    };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { path: resolvedPath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleShellEval(id, params, _config) {
  const command = typeof params.command === "string" ? params.command : "";
  const timeoutMs = typeof params.timeout === "number" ? Math.min(params.timeout, 1e4) : 5e3;
  const span = fsTracer.start({ method: "shell.eval", cmd: command.slice(0, 80) });
  if (!command) {
    span.fail("missing param: command", { method: "shell.eval" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: command" } };
  }
  return new Promise((resolve8) => {
    let stdout = "";
    let stderr = "";
    const child = (0, import_node_child_process5.spawn)("sh", ["-c", command], { env: process.env });
    const timer = setTimeout(() => {
      child.kill();
      span.fail("timed out", { cmd: command.slice(0, 80), timeoutMs });
      resolve8({ jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: "shell.eval timed out" } });
    }, timeoutMs);
    child.stdout.on("data", (d) => {
      stdout += d.toString();
    });
    child.stderr.on("data", (d) => {
      stderr += d.toString();
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      span.ok({ exitCode: code ?? 0, stdoutLen: stdout.length });
      resolve8({ jsonrpc: "2.0", id, result: { stdout, stderr, exitCode: code ?? 0 } });
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      span.fail(err, { cmd: command.slice(0, 80) });
      resolve8({ jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: err.message } });
    });
  });
}
async function handleFsMkdir(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const span = fsTracer.start({ method: "fs.mkdir", path: rawPath || "(empty)" });
  if (!rawPath) {
    span.fail("missing param: path", { method: "fs.mkdir" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  try {
    await (0, import_promises8.mkdir)(absPath, { recursive: true });
    span.ok({ path: absPath });
    return { jsonrpc: "2.0", id, result: { ok: true, path: absPath } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { path: absPath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleFsRmdir(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const recursive = params.recursive === true;
  const span = fsTracer.start({ method: "fs.rmdir", path: rawPath || "(empty)", recursive });
  if (!rawPath) {
    span.fail("missing param: path", { method: "fs.rmdir" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  if (config.workDir.startsWith(absPath) || absPath === "/") {
    span.fail("refusing to remove protected path", { path: absPath });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Refusing to remove protected path" } };
  }
  try {
    if (recursive) {
      await (0, import_promises8.rm)(absPath, { recursive: true, force: true });
    } else {
      await (0, import_promises8.rmdir)(absPath);
    }
    span.ok({ path: absPath, recursive });
    return { jsonrpc: "2.0", id, result: { ok: true, path: absPath } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { path: absPath, recursive });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleFsWatch(id, params, config, notify) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  if (!rawPath) {
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: path" } };
  }
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  const existing = AGENT_WATCH_MAP.get(absPath);
  if (existing) {
    existing.refCount++;
    return { jsonrpc: "2.0", id, result: { ok: true, path: absPath } };
  }
  try {
    const watcher = (0, import_node_fs4.watch)(absPath, { recursive: process.platform !== "linux" }, (eventType, filename) => {
      notify("fs.changed", { path: absPath, eventType, filename: filename ?? null });
    });
    watcher.on("error", (err) => {
      notify("fs.changed", { path: absPath, eventType: "error", filename: null, error: err.message });
      AGENT_WATCH_MAP.delete(absPath);
    });
    AGENT_WATCH_MAP.set(absPath, { watcher, refCount: 1 });
    return { jsonrpc: "2.0", id, result: { ok: true, path: absPath } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: `fs.watch failed: ${msg}` } };
  }
}
async function handleFsUnwatch(id, params, config) {
  const rawPath = typeof params.path === "string" ? params.path : "";
  const absPath = (0, import_node_path6.isAbsolute)(rawPath) ? rawPath : (0, import_node_path6.join)(config.workDir, rawPath);
  const entry = AGENT_WATCH_MAP.get(absPath);
  if (entry) {
    entry.refCount--;
    if (entry.refCount <= 0) {
      entry.watcher.close();
      AGENT_WATCH_MAP.delete(absPath);
    }
  }
  return { jsonrpc: "2.0", id, result: { ok: true } };
}
function cleanupAgentWatches() {
  for (const [path12, entry] of AGENT_WATCH_MAP.entries()) {
    try {
      entry.watcher.close();
    } catch {
    }
    AGENT_WATCH_MAP.delete(path12);
  }
}
var import_promises8, import_node_fs4, import_node_path6, import_node_child_process5, fsTracer, preflightTracer, AGENT_WATCH_MAP;
var init_fs_agent_extensions = __esm({
  "src/relay/fs-agent-extensions.ts"() {
    "use strict";
    import_promises8 = require("node:fs/promises");
    import_node_fs4 = require("node:fs");
    import_node_path6 = require("node:path");
    import_node_child_process5 = require("node:child_process");
    init_agent_wire_protocol();
    init_fs_handler_file_read();
    init_fs_handler_utils();
    init_trace();
    fsTracer = createTracer("agent:fs");
    preflightTracer = createTracer("agent:preflight");
    AGENT_WATCH_MAP = /* @__PURE__ */ new Map();
  }
});

// src/relay/agent-credential-store.ts
var agent_credential_store_exports = {};
__export(agent_credential_store_exports, {
  handleDeleteCredential: () => handleDeleteCredential,
  handleHealthCheck: () => handleHealthCheck,
  handleReadCredential: () => handleReadCredential,
  handleWriteCredential: () => handleWriteCredential,
  readDecryptedKey: () => readDecryptedKey
});
function getCredentialKey() {
  const key = process.env.ORCA_AI_CREDENTIAL_KEY?.trim();
  if (!key) {
    const err = new Error("ORCA_AI_CREDENTIAL_KEY environment variable is not set or empty");
    Object.assign(err, { agentErrorCode: AgentErrorCode.PermissionDenied });
    throw err;
  }
  return key;
}
function credentialFilePath(credentialDir, accountId) {
  if (!/^[\w-]+$/.test(accountId)) {
    const err = new Error(`Invalid accountId: "${accountId}". Only alphanumeric, dash, underscore allowed.`);
    Object.assign(err, { agentErrorCode: AgentErrorCode.InvalidParams });
    throw err;
  }
  return (0, import_node_path7.join)(credentialDir, `${accountId}.enc`);
}
function encryptPayload(masterKey, plaintext) {
  const salt = (0, import_node_crypto.randomBytes)(SALT_BYTES);
  const iv2 = (0, import_node_crypto.randomBytes)(IV_BYTES);
  const key = (0, import_node_crypto.scryptSync)(masterKey, salt, KEY_LEN);
  const cipher = (0, import_node_crypto.createCipheriv)(ALGORITHM, key, iv2);
  const encrypted = Buffer.concat([
    cipher.update(Buffer.from(plaintext, "utf8")),
    cipher.final()
  ]);
  const authTag = cipher.getAuthTag();
  return {
    salt: salt.toString("base64"),
    iv2: iv2.toString("base64"),
    authTag: authTag.toString("base64"),
    data: encrypted.toString("base64")
  };
}
function decryptPayload(masterKey, stored) {
  const salt = Buffer.from(stored.salt, "base64");
  const iv2 = Buffer.from(stored.iv2, "base64");
  const authTag = Buffer.from(stored.authTag, "base64");
  const data = Buffer.from(stored.data, "base64");
  const key = (0, import_node_crypto.scryptSync)(masterKey, salt, KEY_LEN);
  const decipher = (0, import_node_crypto.createDecipheriv)(ALGORITHM, key, iv2);
  decipher.setAuthTag(authTag);
  return Buffer.concat([decipher.update(data), decipher.final()]).toString("utf8");
}
function errorResponse(id, err) {
  const msg = err instanceof Error ? err.message : String(err);
  const code = err.agentErrorCode ?? AgentErrorCode.ServerError;
  return { jsonrpc: "2.0", id, error: { code, message: msg } };
}
async function handleWriteCredential(id, params, config, log) {
  const accountId = typeof params.accountId === "string" ? params.accountId : "";
  const encryptedBlob = typeof params.encryptedBlob === "string" ? params.encryptedBlob : "";
  const iv = typeof params.iv === "string" ? params.iv : "";
  const algorithm = typeof params.algorithm === "string" ? params.algorithm : "AES-GCM";
  const span = credTracer.start({
    method: "ai.provider.writeCredential",
    accountId,
    blobLength: encryptedBlob.length
  });
  if (!accountId || !encryptedBlob || !iv) {
    span.fail("missing required params", { accountId, blobLength: encryptedBlob.length });
    return {
      jsonrpc: "2.0",
      id,
      error: { code: AgentErrorCode.InvalidParams, message: "Missing required params: accountId, encryptedBlob, iv" }
    };
  }
  try {
    const masterKey = getCredentialKey();
    const plaintext = JSON.stringify({ encryptedBlob, iv, algorithm });
    const encrypted = encryptPayload(masterKey, plaintext);
    const stored = { version: FILE_VERSION, ...encrypted };
    (0, import_node_fs5.mkdirSync)(config.credentialDir, { recursive: true, mode: 448 });
    const filePath = credentialFilePath(config.credentialDir, accountId);
    (0, import_node_fs5.writeFileSync)(filePath, JSON.stringify(stored), { mode: 384 });
    log.info(`ai.provider.writeCredential: stored accountId=${accountId}`);
    span.ok({ accountId });
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    log.error(`ai.provider.writeCredential failed: ${err instanceof Error ? err.message : String(err)}`);
    span.fail(err, { accountId });
    return errorResponse(id, err);
  }
}
async function handleReadCredential(id, params, config, log) {
  const accountId = typeof params.accountId === "string" ? params.accountId : "";
  const parentSpanId = typeof params.parentSpanId === "string" ? params.parentSpanId : void 0;
  const span = credTracer.start({ method: "ai.provider.readCredential", accountId, parentSpanId });
  if (!accountId) {
    span.fail("missing accountId", { method: "ai.provider.readCredential" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: accountId" } };
  }
  try {
    const masterKey = getCredentialKey();
    const filePath = credentialFilePath(config.credentialDir, accountId);
    if (!(0, import_node_fs5.existsSync)(filePath)) {
      span.fail("credential not found", { accountId });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.PathNotFound, message: `Credential not found: ${accountId}` } };
    }
    const stored = JSON.parse((0, import_node_fs5.readFileSync)(filePath, "utf8"));
    if (stored.version !== FILE_VERSION) {
      span.fail("unknown version", { accountId, version: stored.version });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: `Unknown credential version: ${stored.version}` } };
    }
    const plaintext = decryptPayload(masterKey, stored);
    const payload = JSON.parse(plaintext);
    span.ok({ accountId });
    return {
      jsonrpc: "2.0",
      id,
      result: {
        accountId,
        encryptedBlob: payload.encryptedBlob,
        iv: payload.iv,
        algorithm: payload.algorithm
      }
    };
  } catch (err) {
    log.error(`ai.provider.readCredential failed: ${err instanceof Error ? err.message : String(err)}`);
    span.fail(err, { accountId });
    return errorResponse(id, err);
  }
}
async function handleHealthCheck(id, params, config, log) {
  const accountId = typeof params.accountId === "string" ? params.accountId : "";
  const provider = typeof params.provider === "string" ? params.provider : "anthropic";
  const start = Date.now();
  const span = credTracer.start({ method: "ai.provider.healthCheck", provider });
  const credResult = await handleReadCredential(id, { accountId }, config, log);
  if (credResult.error) {
    span.fail("credential unreadable", { accountId, provider });
    return {
      jsonrpc: "2.0",
      id,
      error: {
        code: -32001,
        message: "No credential found or decrypt failed. Please re-add the API key in Settings.",
        data: { credentialFound: false, provider }
      }
    };
  }
  const reachability = await checkProviderReachabilityDetailed(provider);
  const latencyMs = Date.now() - start;
  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} ok=${reachability.ok} note=${reachability.note}`);
  if (reachability.ok) {
    span.ok({ provider, latencyMs, note: reachability.note });
  } else {
    span.fail(reachability.note, { provider, latencyMs });
  }
  return {
    jsonrpc: "2.0",
    id,
    result: {
      ok: reachability.ok,
      latencyMs,
      note: reachability.note,
      credentialFound: true,
      // AIP-001: credential exists and decryptable
      ...reachability.statusCode !== void 0 ? { statusCode: reachability.statusCode } : {}
    }
  };
}
async function checkProviderReachabilityDetailed(provider) {
  const url = PROVIDER_HEALTH_URLS[provider];
  if (!url) {return { ok: true, note: "local_provider" };}
  try {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 5e3);
    const resp = await fetch(url, { method: "HEAD", signal: ctrl.signal });
    clearTimeout(timer);
    const statusCode = resp.status;
    if (statusCode < 500) {return { ok: true, note: "reachable", statusCode };}
    return { ok: false, note: "server_error", statusCode };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("abort") || msg.includes("timeout")) {
      return { ok: false, note: "timeout" };
    }
    return { ok: false, note: "unreachable" };
  }
}
async function handleDeleteCredential(id, params, config, log) {
  const accountId = typeof params.accountId === "string" ? params.accountId : "";
  const span = credTracer.start({ method: "ai.provider.deleteCredential", accountId });
  if (!accountId) {
    span.fail("missing accountId", { method: "ai.provider.deleteCredential" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: accountId" } };
  }
  try {
    const filePath = credentialFilePath(config.credentialDir, accountId);
    if (!(0, import_node_fs5.existsSync)(filePath)) {
      log.info(`ai.provider.deleteCredential: not found (idempotent ok) accountId=${accountId}`);
      span.ok({ accountId, deleted: false });
      return { jsonrpc: "2.0", id, result: { ok: true, deleted: false } };
    }
    (0, import_node_fs5.unlinkSync)(filePath);
    log.info(`ai.provider.deleteCredential: deleted accountId=${accountId}`);
    span.ok({ accountId, deleted: true });
    return { jsonrpc: "2.0", id, result: { ok: true, deleted: true } };
  } catch (err) {
    log.error(`ai.provider.deleteCredential failed: ${err instanceof Error ? err.message : String(err)}`);
    span.fail(err, { accountId });
    return errorResponse(id, err);
  }
}
async function readDecryptedKey(accountId, config, log, parentSpanId) {
  const result = await handleReadCredential(null, { accountId, parentSpanId }, config, log);
  if (result.error || !result.result) {return null;}
  return result.result.encryptedBlob;
}
var import_node_crypto, import_node_fs5, import_node_path7, credTracer, ALGORITHM, KEY_LEN, SALT_BYTES, IV_BYTES, FILE_VERSION, PROVIDER_HEALTH_URLS;
var init_agent_credential_store = __esm({
  "src/relay/agent-credential-store.ts"() {
    "use strict";
    import_node_crypto = require("node:crypto");
    import_node_fs5 = require("node:fs");
    import_node_path7 = require("node:path");
    init_agent_wire_protocol();
    init_trace();
    credTracer = createTracer("agent:credential");
    ALGORITHM = "aes-256-gcm";
    KEY_LEN = 32;
    SALT_BYTES = 16;
    IV_BYTES = 12;
    FILE_VERSION = 1;
    PROVIDER_HEALTH_URLS = {
      anthropic: "https://api.anthropic.com",
      openai: "https://api.openai.com",
      gemini: "https://generativelanguage.googleapis.com"
    };
  }
});

// src/relay/external-api-connector.ts
var external_api_connector_exports = {};
__export(external_api_connector_exports, {
  buildGhEnv: () => buildGhEnv,
  buildGlabEnv: () => buildGlabEnv,
  execFileCaptured: () => execFileCaptured,
  handleGitHubAuthStatus: () => handleGitHubAuthStatus,
  handleGitHubIssueCreate: () => handleGitHubIssueCreate,
  handleGitHubIssueList: () => handleGitHubIssueList,
  handleGitHubPrCreate: () => handleGitHubPrCreate,
  handleGitHubPrMerge: () => handleGitHubPrMerge,
  handleGitLabAuthStatus: () => handleGitLabAuthStatus,
  handleGitLabMrCreate: () => handleGitLabMrCreate,
  handleGitLabMrList: () => handleGitLabMrList,
  handleGitLabPipelineStatus: () => handleGitLabPipelineStatus
});
function execFileCaptured(binary, args, opts) {
  return new Promise((resolve8) => {
    const child = (0, import_node_child_process6.spawn)(binary, args, {
      cwd: opts.cwd,
      env: opts.env,
      stdio: ["pipe", "pipe", "pipe"],
      shell: false
      // MANDATORY: no shell injection
    });
    const stdout = [];
    const stderr = [];
    const timer = setTimeout(() => {
      child.kill("SIGTERM");
      resolve8({ stdout: "", stderr: `Timeout after ${opts.timeout}ms`, exitCode: 124 });
    }, opts.timeout);
    child.stdout?.on("data", (b) => stdout.push(b.toString()));
    child.stderr?.on("data", (b) => stderr.push(b.toString()));
    child.on("close", (code) => {
      clearTimeout(timer);
      resolve8({ stdout: stdout.join(""), stderr: stderr.join(""), exitCode: code ?? 0 });
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      resolve8({ stdout: "", stderr: err.message, exitCode: 1 });
    });
    child.stdin?.end();
  });
}
function buildGhEnv(userId, baseEnv) {
  return {
    ...baseEnv,
    GH_CONFIG_DIR: `${(0, import_node_os4.homedir)()}/.config/gh/${userId}/`,
    GH_NO_UPDATE_NOTIFIER: "1",
    GH_PROMPT_DISABLED: "1"
  };
}
function buildGlabEnv(userId, baseEnv) {
  return {
    ...baseEnv,
    GLAB_CONFIG_DIR: `${(0, import_node_os4.homedir)()}/.config/glab-cli/${userId}/`,
    NO_COLOR: "1",
    CI: "1"
  };
}
async function getCurrentBranch(cwd, env) {
  const result = await execFileCaptured("git", ["rev-parse", "--abbrev-ref", "HEAD"], {
    cwd,
    env,
    timeout: 5e3
  });
  return result.exitCode === 0 ? result.stdout.trim() : null;
}
async function checkExistingPr(cwd, branch, env) {
  const result = await execFileCaptured("gh", [
    "pr",
    "list",
    "--head",
    branch,
    "--json",
    "url,number,title,state",
    "--limit",
    "1"
  ], { cwd, env, timeout: 15e3 });
  if (result.exitCode !== 0 || !result.stdout.trim()) {return null;}
  try {
    const prs = JSON.parse(result.stdout);
    return prs[0] ?? null;
  } catch {
    return null;
  }
}
async function handleGitHubPrCreate(id, params, config, log) {
  const title = typeof params.title === "string" ? params.title.trim() : "";
  const body = typeof params.body === "string" ? params.body : "";
  const base = typeof params.base === "string" ? params.base.trim() : "main";
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const draft = params.draft === true;
  const span = apiTracer.start({ method: "github.pr.create", title: title.slice(0, 40), base });
  if (!title) {
    span.fail("missing title", { method: "github.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: title" } };
  }
  if (SHELL_METACHARACTERS2.test(title) || SHELL_METACHARACTERS2.test(base)) {
    span.fail("unsafe characters in params", { method: "github.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in PR params" } };
  }
  const env = buildGhEnv(userId, config.toolEnv);
  const currentBranch = await getCurrentBranch(cwd, env);
  if (currentBranch) {
    const existing = await checkExistingPr(cwd, currentBranch, env);
    if (existing) {
      log.info(`github.pr.create: PR already exists #${existing.number} \u2192 ${existing.url}`);
      span.ok({ prNumber: existing.number, url: existing.url, alreadyExisted: true });
      return { jsonrpc: "2.0", id, result: { ...existing, alreadyExisted: true } };
    }
  }
  const ghArgs = [
    "pr",
    "create",
    "--title",
    title,
    "--body",
    body,
    "--base",
    base,
    "--json",
    "url,number,title,state"
  ];
  if (draft) {ghArgs.push("--draft");}
  try {
    const result = await execFileCaptured("gh", ghArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      log.error(`github.pr.create failed: ${result.stderr}`);
      span.fail(result.stderr || "gh pr create failed", { method: "github.pr.create", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr || "gh pr create failed" } };
    }
    const parsed = JSON.parse(result.stdout);
    log.info(`github.pr.create: PR #${parsed.number} \u2192 ${parsed.url}`);
    span.ok({ prNumber: parsed.number, url: parsed.url });
    return { jsonrpc: "2.0", id, result: parsed };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "github.pr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitHubPrMerge(id, params, config, log) {
  const prNumber = typeof params.prNumber === "number" ? String(params.prNumber) : "";
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const method = typeof params.method === "string" ? params.method : "squash";
  const span = apiTracer.start({ method: "github.pr.merge", prNumber: prNumber || "(empty)", mergeMethod: method });
  if (!prNumber) {
    span.fail("missing prNumber", { method: "github.pr.merge" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: prNumber" } };
  }
  const mergeFlag = method === "rebase" ? "--rebase" : method === "merge" ? "--merge" : "--squash";
  const ghArgs = ["pr", "merge", prNumber, mergeFlag, "--auto"];
  const env = buildGhEnv(userId, config.toolEnv);
  try {
    const result = await execFileCaptured("gh", ghArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "gh pr merge failed", { prNumber, exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr || "gh pr merge failed" } };
    }
    log.info(`github.pr.merge: PR #${prNumber} merged`);
    span.ok({ prNumber, method });
    return { jsonrpc: "2.0", id, result: { ok: true, stdout: result.stdout } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { prNumber, method });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitHubIssueList(id, params, config, log) {
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const limit = typeof params.limit === "number" ? Math.min(params.limit, 50) : 30;
  const state = typeof params.state === "string" ? params.state : "open";
  const span = apiTracer.start({ method: "github.issue.list", state, limit });
  const env = buildGhEnv(userId, config.toolEnv);
  const ghArgs = ["issue", "list", "--json", "number,title,state,url", "--limit", String(limit), "--state", state];
  try {
    const result = await execFileCaptured("gh", ghArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "gh issue list failed", { method: "github.issue.list", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr } };
    }
    const issues = JSON.parse(result.stdout);
    log.info(`github.issue.list: ${issues.length} issues`);
    span.ok({ total: issues.length });
    return { jsonrpc: "2.0", id, result: { issues, total: issues.length } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "github.issue.list" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitHubIssueCreate(id, params, config, log) {
  const title = typeof params.title === "string" ? params.title.trim() : "";
  const body = typeof params.body === "string" ? params.body : "";
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const span = apiTracer.start({ method: "github.issue.create", title: title.slice(0, 40) });
  if (!title) {
    span.fail("missing title", { method: "github.issue.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: title" } };
  }
  if (SHELL_METACHARACTERS2.test(title)) {
    span.fail("unsafe characters in params", { method: "github.issue.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in issue title" } };
  }
  const env = buildGhEnv(userId, config.toolEnv);
  const ghArgs = ["issue", "create", "--title", title, "--body", body, "--json", "number,url,title"];
  try {
    const result = await execFileCaptured("gh", ghArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "gh issue create failed", { method: "github.issue.create", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr } };
    }
    const parsed = JSON.parse(result.stdout);
    log.info(`github.issue.create: issue #${parsed.number} \u2192 ${parsed.url}`);
    span.ok({ issueNumber: parsed.number, url: parsed.url });
    return { jsonrpc: "2.0", id, result: parsed };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "github.issue.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitHubAuthStatus(id, params, config, log) {
  const userId = typeof params.userId === "string" ? params.userId : "";
  const env = buildGhEnv(userId, config.toolEnv);
  const span = apiTracer.start({ method: "github.auth.status", cli: "gh" });
  span.step("exec", { cli: "gh" });
  try {
    const result = await execFileCaptured("gh", ["auth", "status"], {
      cwd: config.workDir,
      env,
      timeout: 1e4
    });
    const ok = result.exitCode === 0;
    log.info(`github.auth.status: userId=${userId} ok=${ok}`);
    if (ok) {
      span.ok({ cli: "gh", authenticated: ok });
    } else {
      span.fail(result.stderr || "gh auth status non-zero exit", { cli: "gh", exitCode: result.exitCode, authenticated: false });
    }
    return { jsonrpc: "2.0", id, result: { ok, stdout: result.stdout, stderr: result.stderr } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { cli: "gh" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitLabMrCreate(id, params, config, log) {
  const title = typeof params.title === "string" ? params.title.trim() : "";
  const description = typeof params.description === "string" ? params.description : "";
  const targetBranch = typeof params.targetBranch === "string" ? params.targetBranch.trim() : "main";
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const span = apiTracer.start({ method: "gitlab.mr.create", title: title.slice(0, 40), targetBranch });
  if (!title) {
    span.fail("missing title", { method: "gitlab.mr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing required param: title" } };
  }
  if (SHELL_METACHARACTERS2.test(title) || SHELL_METACHARACTERS2.test(targetBranch)) {
    span.fail("unsafe characters in params", { method: "gitlab.mr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Unsafe characters in MR params" } };
  }
  const glabArgs = [
    "mr",
    "create",
    "--title",
    title,
    "--description",
    description,
    "--target-branch",
    targetBranch,
    "--yes"
    // non-interactive
  ];
  const env = buildGlabEnv(userId, config.toolEnv);
  try {
    const result = await execFileCaptured("glab", glabArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "glab mr create failed", { method: "gitlab.mr.create", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr || "glab mr create failed" } };
    }
    const url = result.stdout.trim().split("\n").find((l) => l.startsWith("https://")) ?? result.stdout.trim();
    log.info(`gitlab.mr.create: MR \u2192 ${url}`);
    span.ok({ url });
    return { jsonrpc: "2.0", id, result: { url, stdout: result.stdout, stderr: result.stderr } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "gitlab.mr.create" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitLabMrList(id, params, config, log) {
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const state = typeof params.state === "string" ? params.state : "opened";
  const span = apiTracer.start({ method: "gitlab.mr.list", state });
  const env = buildGlabEnv(userId, config.toolEnv);
  const glabArgs = ["mr", "list", "--state", state, "--output", "json"];
  try {
    const result = await execFileCaptured("glab", glabArgs, { cwd, env, timeout: 3e4 });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "glab mr list failed", { method: "gitlab.mr.list", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr } };
    }
    const mrs = JSON.parse(result.stdout);
    log.info(`gitlab.mr.list: ${mrs.length} MRs state=${state}`);
    span.ok({ total: mrs.length });
    return { jsonrpc: "2.0", id, result: { mrs, total: mrs.length } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "gitlab.mr.list" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitLabPipelineStatus(id, params, config, log) {
  const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
  const userId = typeof params.userId === "string" ? params.userId : "";
  const span = apiTracer.start({ method: "gitlab.pipeline.status" });
  const env = buildGlabEnv(userId, config.toolEnv);
  try {
    const result = await execFileCaptured("glab", ["pipeline", "status", "--output", "json"], {
      cwd,
      env,
      timeout: 3e4
    });
    if (result.exitCode !== 0) {
      span.fail(result.stderr || "glab pipeline status failed", { method: "gitlab.pipeline.status", exitCode: result.exitCode });
      return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: result.stderr } };
    }
    const status = JSON.parse(result.stdout);
    log.info(`gitlab.pipeline.status: ok`);
    span.ok({});
    return { jsonrpc: "2.0", id, result: { status, raw: result.stdout } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { method: "gitlab.pipeline.status" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
async function handleGitLabAuthStatus(id, params, config, log) {
  const userId = typeof params.userId === "string" ? params.userId : "";
  const env = buildGlabEnv(userId, config.toolEnv);
  const span = apiTracer.start({ method: "gitlab.auth.status", cli: "glab" });
  span.step("exec", { cli: "glab" });
  try {
    const result = await execFileCaptured("glab", ["auth", "status"], {
      cwd: config.workDir,
      env,
      timeout: 1e4
    });
    const ok = result.exitCode === 0;
    log.info(`gitlab.auth.status: userId=${userId} ok=${ok}`);
    if (ok) {
      span.ok({ cli: "glab", authenticated: ok });
    } else {
      span.fail(result.stderr || "glab auth status non-zero exit", { cli: "glab", exitCode: result.exitCode, authenticated: false });
    }
    return { jsonrpc: "2.0", id, result: { ok, stdout: result.stdout, stderr: result.stderr } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { cli: "glab" });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
var import_node_child_process6, import_node_os4, apiTracer, SHELL_METACHARACTERS2;
var init_external_api_connector = __esm({
  "src/relay/external-api-connector.ts"() {
    "use strict";
    import_node_child_process6 = require("node:child_process");
    import_node_os4 = require("node:os");
    init_agent_wire_protocol();
    init_trace();
    apiTracer = createTracer("agent:ext-api");
    SHELL_METACHARACTERS2 = /[&|;$`<>\\!]/;
  }
});

// src/relay/agent-spawner.ts
var agent_spawner_exports = {};
__export(agent_spawner_exports, {
  SubAgentSpawner: () => SubAgentSpawner,
  buildAgentEnv: () => buildAgentEnv,
  cleanupAllPtys: () => cleanupAllPtys,
  handleAgentKill: () => handleAgentKill,
  handleAgentSendInput: () => handleAgentSendInput,
  handleAgentSpawn: () => handleAgentSpawn,
  resolveAgentSpec: () => resolveAgentSpec
});
function extractResume(params) {
  const t = params["_trace"];
  if (t && typeof t === "object" && typeof t.id === "string") {
    return { id: t.id };
  }
  return void 0;
}
function resolveAgentSpec(modelId) {
  if (!modelId) {return void 0;}
  for (const [prefix, idx] of MODEL_PREFIX_MAP) {
    if (modelId === prefix || modelId.startsWith(`${prefix  }-`) || modelId.startsWith(prefix)) {
      return AGENT_SPECS[idx];
    }
  }
  return void 0;
}
function buildAgentArgs(spec, req) {
  return spec.buildArgs({ resumeId: req.resumeId });
}
async function buildAgentEnv(req, spec, config, resolvedApiKey, log, parentSpanId) {
  const accountId = "accountId" in req ? req.accountId : "";
  const userId = "userId" in req ? req.userId : "";
  const taskId = "taskId" in req ? req.taskId : "";
  const projectId = "projectId" in req ? req.projectId ?? "" : "";
  const cwd = "cwd" in req ? req.cwd ?? config.workDir : config.workDir;
  const base = {
    HOME: process.env.HOME ?? "/tmp",
    PATH: config.toolPath ?? process.env.PATH ?? "/usr/bin:/bin",
    TERM: "xterm-256color",
    ORCA_AGENT_CWD: cwd,
    ORCA_ACCOUNT_ID: accountId,
    ORCA_TASK_ID: taskId,
    ORCA_USER_ID: userId,
    ...projectId ? { ORCA_PROJECT_ID: projectId } : {},
    // Per-user GitHub/GitLab config dirs to isolate credentials across agents
    GH_CONFIG_DIR: `${process.env.HOME ?? "/tmp"}/.config/gh/${userId}/`,
    GLAB_CONFIG_DIR: `${process.env.HOME ?? "/tmp"}/.config/glab-cli/${userId}/`
  };
  if (resolvedApiKey) {
    base["ANTHROPIC_API_KEY"] = resolvedApiKey;
    base["OPENAI_API_KEY"] = resolvedApiKey;
    base["GEMINI_API_KEY"] = resolvedApiKey;
  } else if (spec.apiKeyEnvVar && accountId) {
    const logFn = log ?? {
      info: () => {
      },
      warn: () => {
      },
      error: () => {
      },
      debug: () => {
      }
    };
    const blob = await readDecryptedKey(accountId, config, logFn, parentSpanId);
    if (blob) {
      base[spec.apiKeyEnvVar] = blob;
      logFn.warn?.(`buildAgentEnv: injecting Layer1 blob for ${spec.apiKeyEnvVar} \u2014 agent may fail auth if key not plaintext`);
    } else {
      logFn.warn?.(`buildAgentEnv: no credential found for accountId=${accountId} \u2014 agent will fail authentication`);
    }
  }
  if (spec.localInference) {
    base.OLLAMA_HOST = process.env.OLLAMA_HOST ?? "http://localhost:11434";
    base.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL ?? "http://localhost:8000/v1";
  }
  const extra = "extraEnv" in req ? req.extraEnv ?? {} : {};
  return { ...base, ...extra };
}
async function handleAgentSpawn(id, params, config, log, ws, _state) {
  const wireState = createWireState();
  const modelId = typeof params.model === "string" ? params.model : typeof params.modelId === "string" ? params.modelId : "";
  const req = {
    taskId: typeof params.taskId === "string" ? params.taskId : "",
    userId: typeof params.userId === "string" ? params.userId : "",
    modelId,
    accountId: typeof params.accountId === "string" ? params.accountId : "",
    cwd: typeof params.cwd === "string" ? params.cwd : void 0,
    resumeId: typeof params.resumeId === "string" ? params.resumeId : void 0,
    worktreePath: typeof params.worktreePath === "string" ? params.worktreePath : void 0,
    branchName: typeof params.branchName === "string" ? params.branchName : void 0
  };
  const resolvedApiKey = typeof params.resolvedApiKey === "string" ? params.resolvedApiKey : void 0;
  const span = spawnerTracer.start({ method: "agent.spawn", taskId: req.taskId, modelId: req.modelId });
  const orchTracer = req.resumeId ? Tracers.agentOrchResume : Tracers.agentOrchSpawn;
  const orchSpan = orchTracer.start(
    { taskId: req.taskId, modelId: req.modelId, resumeId: req.resumeId },
    extractResume(params)
  );
  const missing = [];
  if (!req.modelId) {missing.push("model");}
  if (!req.taskId) {missing.push("taskId");}
  if (!req.userId) {missing.push("userId");}
  if (!req.cwd) {missing.push("cwd");}
  if (missing.length > 0) {
    span.fail(`missing ${missing.join(",")}`, { taskId: req.taskId, modelId: req.modelId });
    orchSpan.fail(`missing ${missing.join(",")}`, { taskId: req.taskId });
    const errResp = {
      jsonrpc: "2.0",
      id,
      error: { code: AgentErrorCode.InvalidParams, message: `Missing required fields: ${missing.join(", ")}` }
    };
    try {
      ws.send(encodeDataFrame(wireState, JSON.stringify(errResp)));
    } catch {
    }
    return errResp;
  }
  const specResolved = resolveAgentSpec(req.modelId);
  if (!specResolved) {
    span.fail("unknown model", { modelId: req.modelId });
    orchSpan.fail("unknown model", { modelId: req.modelId });
    const errResp = {
      jsonrpc: "2.0",
      id,
      error: { code: AgentErrorCode.InvalidParams, message: `Unknown model: ${req.modelId}` }
    };
    try {
      ws.send(encodeDataFrame(wireState, JSON.stringify(errResp)));
    } catch {
    }
    return errResp;
  }
  const spawner = new SubAgentSpawner();
  try {
    spawner.transition("spawning");
    const spec = specResolved;
    orchSpan.step("resolve-credential", { accountId: req.accountId || "(none)" });
    const envBase = await buildAgentEnv(
      req,
      spec,
      config,
      resolvedApiKey ?? null,
      log,
      span.id
      // NEW — CR-TRACE-016 correlation field (spawnerTracer span, NOT orchSpan)
    );
    const env = {
      ...envBase,
      ...req.worktreePath ? { ORCA_WORKTREE_PATH: req.worktreePath } : {},
      ...req.branchName ? { ORCA_WORKTREE_BRANCH: req.branchName } : {}
    };
    const ptyId = `pty-${req.userId}-${req.taskId}-${Date.now()}`;
    const args = buildAgentArgs(spec, req);
    const { existsSync: fsExistsSync } = await import("node:fs");
    const { join: pathJoin } = await import("node:path");
    const toolPathDirs = (config.toolPath ?? "").split(":").filter(Boolean);
    const binaryExists = process.platform === "win32" ? true : toolPathDirs.some((dir) => fsExistsSync(pathJoin(dir, spec.binary))) || !toolPathDirs.length;
    if (!binaryExists) {
      throw new Error(
        `Agent binary '${spec.binary}' not found in toolPath '${config.toolPath ?? "(empty)"}'. Install it or set toolPath to the directory containing '${spec.binary}'.`
      );
    }
    let nodePty;
    try {
      nodePty = await import("node-pty");
    } catch {
      throw new Error(
        `node-pty is not installed on this dev server. Run: npm install node-pty  (in ~/orca-agent/)  to enable PTY-based agent spawning.`
      );
    }
    orchSpan.step("node-pty-spawn", { binary: spec.binary, ptyId });
    const pty = nodePty.spawn(spec.binary, args, {
      name: "xterm-256color",
      cols: 220,
      rows: 50,
      cwd: req.cwd ?? config.workDir,
      env
    });
    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId });
    spawner.transition("running");
    span.step("pty-running", { ptyId, modelId: req.modelId });
    log.info(`agent.spawn: ptyId=${ptyId} model=${req.modelId}`);
    let firstOutputReported = false;
    pty.onData((data) => {
      if (!firstOutputReported) {
        firstOutputReported = true;
        orchSpan.step("first-output", { ptyId });
      }
      const notification = JSON.stringify({
        jsonrpc: "2.0",
        method: "agent.output",
        params: { ptyId, data: Buffer.from(data).toString("base64") }
      });
      ws.send(encodeDataFrame(wireState, notification));
    });
    pty.onExit(({ exitCode }) => {
      PTY_REGISTRY.delete(ptyId);
      spawner.transition("stopping");
      spawner.transition("stopped");
      if (exitCode === 0) {
        span.ok({ ptyId, exitCode });
        orchSpan.ok({ ptyId, exitCode });
      } else {
        span.fail(`exit code ${exitCode}`, { ptyId, exitCode });
        orchSpan.fail(`exit code ${exitCode}`, { ptyId, exitCode });
      }
      const notification = JSON.stringify({
        jsonrpc: "2.0",
        method: "agent.exited",
        params: { ptyId, exitCode }
      });
      ws.send(encodeDataFrame(wireState, notification));
      log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`);
    });
    return { jsonrpc: "2.0", id, result: { ok: true, ptyId } };
  } catch (err) {
    spawner.transition("error");
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { taskId: req.taskId, modelId: req.modelId });
    orchSpan.fail(err, { taskId: req.taskId, modelId: req.modelId });
    log.error(`agent.spawn: error ${msg}`);
    const errWireState = createWireState();
    const errResp = { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
    ws.send(encodeDataFrame(errWireState, JSON.stringify(errResp)));
    return errResp;
  }
}
async function handleAgentKill(id, params, _config, log) {
  const ptyId = typeof params.ptyId === "string" ? params.ptyId : "";
  const rawSignal = typeof params.signal === "string" ? params.signal : "SIGTERM";
  const signal = rawSignal === "SIGKILL" ? "SIGKILL" : "SIGTERM";
  const span = spawnerTracer.start({ method: "agent.kill", ptyId: ptyId || "(empty)", signal });
  const orchSpan = Tracers.agentOrchStop.start({ ptyId: ptyId || "(empty)", signal, via: "agent.kill" }, extractResume(params));
  if (!ptyId) {
    span.fail("missing ptyId", { method: "agent.kill" });
    orchSpan.fail("missing ptyId");
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing ptyId" } };
  }
  const entry = PTY_REGISTRY.get(ptyId);
  if (!entry) {
    span.ok({ ptyId, note: "already dead" });
    orchSpan.ok({ ptyId, note: "already dead" });
    return { jsonrpc: "2.0", id, result: { ok: true, note: "pty not found (already dead)" } };
  }
  if (process.platform === "win32") {
    entry.pty.kill();
  } else {
    entry.pty.kill(signal);
  }
  PTY_REGISTRY.delete(ptyId);
  span.ok({ ptyId, signal });
  orchSpan.ok({ ptyId, signal });
  log.info(`agent.kill: ptyId=${ptyId} ${signal} sent`);
  return { jsonrpc: "2.0", id, result: { ok: true } };
}
async function handleAgentSendInput(id, params, _config, log) {
  const ptyId = typeof params.ptyId === "string" ? params.ptyId : "";
  const data = typeof params.data === "string" ? params.data : "";
  const isGracefulStop = data === "";
  const orchSpan = isGracefulStop ? Tracers.agentOrchStop.start({ ptyId, via: "agent.sendInput" }, extractResume(params)) : void 0;
  const span = spawnerTracer.start({ method: "agent.sendInput", ptyId: ptyId || "(empty)" });
  if (!ptyId) {
    span.fail("missing ptyId", { method: "agent.sendInput" });
    orchSpan?.fail("missing ptyId");
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.InvalidParams, message: "Missing ptyId" } };
  }
  const entry = PTY_REGISTRY.get(ptyId);
  if (!entry) {
    span.fail("pty-not-found", { ptyId });
    orchSpan?.fail("pty not found", { ptyId });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } };
  }
  try {
    entry.pty.write(data);
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`);
    span.ok({ ptyId, bytes: data.length });
    orchSpan?.ok({ ptyId });
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    log.error(`agent.sendInput failed: ${msg}`);
    span.fail(err, { ptyId });
    orchSpan?.fail(err, { ptyId });
    return { jsonrpc: "2.0", id, error: { code: AgentErrorCode.ServerError, message: msg } };
  }
}
function cleanupAllPtys(log) {
  if (PTY_REGISTRY.size === 0) {return;}
  log.info(`session.stop: cleaning up ${PTY_REGISTRY.size} orphaned PTY(s)`);
  for (const [ptyId, entry] of PTY_REGISTRY.entries()) {
    try {
      if (process.platform === "win32") {
        entry.pty.kill();
      } else {
        entry.pty.kill("SIGTERM");
      }
      log.info(`session.stop: killed PTY ${ptyId}`);
    } catch (err) {
      log.warn(`session.stop: failed to kill PTY ${ptyId}: ${err}`);
    }
  }
  PTY_REGISTRY.clear();
}
var spawnerTracer, PTY_REGISTRY, SubAgentSpawner, AGENT_SPECS, MODEL_PREFIX_MAP;
var init_agent_spawner = __esm({
  "src/relay/agent-spawner.ts"() {
    "use strict";
    init_agent_wire();
    init_trace();
    init_tracers();
    init_agent_credential_store();
    init_agent_wire_protocol();
    spawnerTracer = createTracer("agent:spawn");
    PTY_REGISTRY = /* @__PURE__ */ new Map();
    SubAgentSpawner = class {
      state = "idle";
      getState() {
        return this.state;
      }
      transition(next) {
        const VALID = {
          idle: ["spawning"],
          spawning: ["running", "error"],
          running: ["stopping", "error"],
          stopping: ["stopped", "error"],
          stopped: ["idle"],
          error: ["idle"]
        };
        if (!VALID[this.state]?.includes(next)) {
          throw new Error(`SubAgentSpawner: invalid transition ${this.state} \u2192 ${next}`);
        }
        this.state = next;
      }
    };
    AGENT_SPECS = [
      // index 0: claude — output-format stream-json for automation; --verbose for tracing
      {
        binary: "claude",
        buildArgs: (req) => req?.resumeId ? ["--resume", req.resumeId] : ["--output-format", "stream-json", "--verbose"],
        apiKeyEnvVar: "ANTHROPIC_API_KEY"
      },
      // index 1: codex / openai compatible
      {
        binary: "codex",
        buildArgs: (req) => req?.resumeId ? ["--session-file", `~/.codex/${req.resumeId}.json`] : [],
        apiKeyEnvVar: "OPENAI_API_KEY"
      },
      // index 2: gemini
      { binary: "gemini", buildArgs: () => ["--stream"], apiKeyEnvVar: "GEMINI_API_KEY" },
      // index 3: opencode — no API key needed (uses its own auth)
      { binary: "opencode", buildArgs: () => [], apiKeyEnvVar: null },
      // index 4: ollama — local inference, no external API key
      { binary: "ollama", buildArgs: () => [], apiKeyEnvVar: null, localInference: true }
    ];
    MODEL_PREFIX_MAP = [
      ["claude", 0],
      ["gpt-", 1],
      ["codex", 1],
      ["gemini", 2],
      ["opencode", 3],
      ["ollama", 4]
      // matches 'ollama' and 'ollama-*'
    ];
  }
});

// src/relay/ai-complete-handler.ts
var ai_complete_handler_exports = {};
__export(ai_complete_handler_exports, {
  handleAIComplete: () => handleAIComplete
});
async function handleAIComplete(params, config, log) {
  const { prompt, format = "text", taskId } = params;
  const span = aiCompleteTracer.start({
    method: "ai.complete",
    format,
    taskId,
    promptLength: prompt.length
  });
  if (!prompt.trim()) {
    span.fail("empty prompt", { taskId });
    throw new Error("ai.complete: prompt must not be empty");
  }
  const model = params.model ?? config["defaultModel"] ?? process.env["ORCA_AI_MODEL_ID"] ?? "claude-opus-4-5";
  const apiKey = resolveApiKey(model);
  if (!apiKey) {
    span.fail("no API key for model", { model, taskId });
    throw new Error(
      `ai.complete: No API key found for model "${model}". Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY in the agent environment, or configure an AI provider in Orca settings.`
    );
  }
  log.info(`ai.complete: model=${model} format=${format} promptLen=${prompt.length}`);
  span.step("provider-call", { model, provider: providerNameFromModel(model) });
  try {
    const text = await dispatch(model, apiKey, prompt, format, log);
    span.ok({ model, contentLength: text.length });
    return { content: text, model };
  } catch (err) {
    span.fail(err, { model, taskId });
    throw err;
  }
}
function providerNameFromModel(model) {
  if (model.startsWith("claude")) {return "anthropic";}
  if (model.startsWith("gpt") || model.startsWith("o1") || model.startsWith("o3") || model.startsWith("o4")) {return "openai";}
  if (model.startsWith("gemini")) {return "google";}
  return "unknown";
}
function resolveApiKey(model) {
  if (model.startsWith("claude")) {
    return process.env["ANTHROPIC_API_KEY"] ?? null;
  }
  if (model.startsWith("gpt") || model.startsWith("o1") || model.startsWith("o3") || model.startsWith("o4")) {
    return process.env["OPENAI_API_KEY"] ?? null;
  }
  if (model.startsWith("gemini")) {
    return process.env["GOOGLE_API_KEY"] ?? null;
  }
  return process.env["ANTHROPIC_API_KEY"] ?? process.env["OPENAI_API_KEY"] ?? process.env["GOOGLE_API_KEY"] ?? null;
}
async function dispatch(model, apiKey, prompt, format, log) {
  if (model.startsWith("claude")) {
    return callAnthropic(model, apiKey, prompt, format, log);
  }
  if (model.startsWith("gpt") || model.startsWith("o1") || model.startsWith("o3") || model.startsWith("o4")) {
    return callOpenAI(model, apiKey, prompt, log);
  }
  if (model.startsWith("gemini")) {
    return callGoogle(model, apiKey, prompt, log);
  }
  throw new Error(`ai.complete: Unknown model provider for model "${model}". Supported prefixes: claude, gpt, o1, o3, o4, gemini.`);
}
async function callAnthropic(model, apiKey, prompt, format, log) {
  const body = {
    model,
    max_tokens: 4096,
    messages: [{ role: "user", content: prompt }]
  };
  if (format === "json") {
    body["system"] = "Respond with valid JSON only. No markdown code fences, no explanation outside the JSON object.";
  }
  const res = await fetch("https://api.anthropic.com/v1/messages", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "x-api-key": apiKey,
      "anthropic-version": "2023-06-01"
    },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(12e4)
  });
  if (!res.ok) {
    const errBody = await res.text().catch(() => res.statusText);
    log.error(`ai.complete Anthropic ${res.status}: ${errBody}`);
    throw new Error(`Anthropic API error ${res.status}: ${errBody}`);
  }
  const data = await res.json();
  return data.content.find((c) => c.type === "text")?.text ?? "";
}
async function callOpenAI(model, apiKey, prompt, log) {
  const res = await fetch("https://api.openai.com/v1/chat/completions", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${apiKey}`
    },
    body: JSON.stringify({
      model,
      messages: [{ role: "user", content: prompt }],
      max_tokens: 4096
    }),
    signal: AbortSignal.timeout(12e4)
  });
  if (!res.ok) {
    const errBody = await res.text().catch(() => res.statusText);
    log.error(`ai.complete OpenAI ${res.status}: ${errBody}`);
    throw new Error(`OpenAI API error ${res.status}: ${errBody}`);
  }
  const data = await res.json();
  return data.choices[0]?.message.content ?? "";
}
async function callGoogle(model, apiKey, prompt, log) {
  const url = `https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent?key=${apiKey}`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      contents: [{ parts: [{ text: prompt }] }]
    }),
    signal: AbortSignal.timeout(12e4)
  });
  if (!res.ok) {
    const errBody = await res.text().catch(() => res.statusText);
    log.error(`ai.complete Google ${res.status}: ${errBody}`);
    throw new Error(`Google AI API error ${res.status}: ${errBody}`);
  }
  const data = await res.json();
  return data.candidates[0]?.content.parts[0]?.text ?? "";
}
var aiCompleteTracer;
var init_ai_complete_handler = __esm({
  "src/relay/ai-complete-handler.ts"() {
    "use strict";
    init_trace();
    aiCompleteTracer = createTracer("agent:aiComplete");
  }
});

// src/relay/pty-daemon-protocol.ts
function isDaemonRequest(msg) {
  return "id" in msg && "method" in msg;
}
function isDaemonResponse(msg) {
  return "id" in msg && !("method" in msg);
}
function encodeDaemonMessage(msg) {
  return `${JSON.stringify(msg)}
`;
}
var DaemonMessageDecoder;
var init_pty_daemon_protocol = __esm({
  "src/relay/pty-daemon-protocol.ts"() {
    "use strict";
    DaemonMessageDecoder = class {
      constructor(onMessage) {
        this.onMessage = onMessage;
      }
      onMessage;
      buffer = "";
      feed(chunk) {
        this.buffer += chunk;
        let newlineIndex = this.buffer.indexOf("\n");
        while (newlineIndex !== -1) {
          const line = this.buffer.slice(0, newlineIndex);
          this.buffer = this.buffer.slice(newlineIndex + 1);
          if (line.trim()) {
            try {
              this.onMessage(JSON.parse(line));
            } catch {
            }
          }
          newlineIndex = this.buffer.indexOf("\n");
        }
      }
    };
  }
});

// src/relay/pty-daemon-client.ts
var pty_daemon_client_exports = {};
__export(pty_daemon_client_exports, {
  getDaemonSocketPath: () => getDaemonSocketPath,
  handlePtyAttach: () => handlePtyAttach,
  handlePtyCreate: () => handlePtyCreate,
  handlePtyDestroy: () => handlePtyDestroy,
  handlePtyResize: () => handlePtyResize,
  handlePtyScrollback: () => handlePtyScrollback,
  handlePtySendSignal: () => handlePtySendSignal,
  handlePtyWrite: () => handlePtyWrite,
  notifyDaemonSessionClosed: () => notifyDaemonSessionClosed
});
function getDaemonSocketPath() {
  return path11.join(os.homedir(), "orca-agent", "pty-daemon.sock");
}
function tryConnect(socketPath, timeoutMs) {
  return new Promise((resolve8, reject) => {
    const sock = net.createConnection(socketPath);
    const timer = setTimeout(() => {
      sock.destroy();
      reject(new Error(`pty-daemon connect timed out after ${timeoutMs}ms`));
    }, timeoutMs);
    sock.once("connect", () => {
      clearTimeout(timer);
      resolve8(sock);
    });
    sock.once("error", (err) => {
      clearTimeout(timer);
      reject(err);
    });
  });
}
function getDaemonLogPath() {
  return path11.join(os.homedir(), "orca-agent", "logs", "pty-daemon.log");
}
function spawnDaemon(socketPath, log) {
  log.info(`pty-daemon-client: spawning daemon (socket=${socketPath})`);
  const logPath = getDaemonLogPath();
  fs.mkdirSync(path11.dirname(logPath), { recursive: true });
  const logFd = fs.openSync(logPath, "a");
  const child = (0, import_node_child_process7.spawn)(process.execPath, [process.argv[1]], {
    env: { ...process.env, ORCA_PTY_DAEMON_SOCKET: socketPath },
    detached: true,
    stdio: ["ignore", logFd, logFd]
  });
  child.unref();
  fs.closeSync(logFd);
}
async function waitForDaemonReady(socketPath, log) {
  const deadline = Date.now() + SPAWN_WAIT_TIMEOUT_MS;
  let lastErr = null;
  while (Date.now() < deadline) {
    try {
      return await tryConnect(socketPath, CONNECT_TIMEOUT_MS);
    } catch (err) {
      lastErr = err;
      await new Promise((r) => setTimeout(r, SPAWN_WAIT_POLL_MS));
    }
  }
  const msg = lastErr instanceof Error ? lastErr.message : String(lastErr);
  log.error(`pty-daemon-client: daemon never became ready: ${msg}`);
  throw new Error(`pty-daemon did not become ready: ${msg}`);
}
function wireSocket(sock) {
  const decoder = new DaemonMessageDecoder((msg) => {
    if (isDaemonResponse(msg)) {
      const pending = pendingRequests.get(msg.id);
      if (!pending) {return;}
      pendingRequests.delete(msg.id);
      if (msg.error) {pending.reject(new Error(msg.error.message));}
      else {pending.resolve({ result: msg.result });}
      return;
    }
    if ("method" in msg && !("id" in msg)) {
      currentNotify?.(msg.method, msg.params);
    }
  });
  sock.on("data", (chunk) => decoder.feed(chunk.toString("utf8")));
  const onDrop = () => {
    if (socket === sock) {socket = null;}
    for (const [id, pending] of pendingRequests) {
      pending.reject(new Error("pty-daemon connection lost"));
      pendingRequests.delete(id);
    }
  };
  sock.on("close", onDrop);
  sock.on("error", onDrop);
}
async function ensureConnection(log) {
  if (socket && !socket.destroyed) {return socket;}
  if (connectingPromise) {return connectingPromise;}
  connectingPromise = (async () => {
    const socketPath = getDaemonSocketPath();
    let sock;
    try {
      sock = await tryConnect(socketPath, CONNECT_TIMEOUT_MS);
    } catch {
      spawnDaemon(socketPath, log);
      sock = await waitForDaemonReady(socketPath, log);
    }
    wireSocket(sock);
    socket = sock;
    return sock;
  })();
  try {
    return await connectingPromise;
  } finally {
    connectingPromise = null;
  }
}
async function sendRequest(method, params, log) {
  const sock = await ensureConnection(log);
  const id = nextRequestId++;
  return new Promise((resolve8, reject) => {
    const timer = setTimeout(() => {
      pendingRequests.delete(id);
      reject(new Error(`pty-daemon request '${method}' timed out after ${REQUEST_TIMEOUT_MS}ms`));
    }, REQUEST_TIMEOUT_MS);
    pendingRequests.set(id, {
      resolve: (v) => {
        clearTimeout(timer);
        resolve8(v);
      },
      reject: (err) => {
        clearTimeout(timer);
        reject(err);
      }
    });
    sock.write(encodeDaemonMessage({ id, method, params }));
  });
}
async function forward(method, id, params, log, notify) {
  if (notify) {currentNotify = notify;}
  try {
    const outcome = await sendRequest(method, params, log);
    if (outcome.error) {
      return { jsonrpc: "2.0", id, error: { code: -32603, message: outcome.error.message } };
    }
    return { jsonrpc: "2.0", id, result: outcome.result };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { jsonrpc: "2.0", id, error: { code: -32603, message: msg } };
  }
}
async function handlePtyCreate(id, params, log, notify) {
  return forward("pty.create", id, params, log, notify);
}
async function handlePtyAttach(id, params, log, notify) {
  return forward("pty.attach", id, params, log, notify);
}
async function handlePtyWrite(id, params, log, notify) {
  return forward("pty.write", id, params, log, notify);
}
async function handlePtyResize(id, params, log, notify) {
  return forward("pty.resize", id, params, log, notify);
}
async function handlePtyDestroy(id, params, log, notify) {
  return forward("pty.destroy", id, params, log, notify);
}
async function handlePtyScrollback(id, params, log, notify) {
  return forward("pty.scrollback", id, params, log, notify);
}
async function handlePtySendSignal(id, params, log, notify) {
  return forward("pty.sendSignal", id, params, log, notify);
}
async function notifyDaemonSessionClosed(log) {
  try {
    await sendRequest("daemon.sessionClosed", {}, log);
  } catch {
  }
}
var net, path11, os, fs, import_node_child_process7, REQUEST_TIMEOUT_MS, CONNECT_TIMEOUT_MS, SPAWN_WAIT_TIMEOUT_MS, SPAWN_WAIT_POLL_MS, socket, connectingPromise, nextRequestId, pendingRequests, currentNotify;
var init_pty_daemon_client = __esm({
  "src/relay/pty-daemon-client.ts"() {
    "use strict";
    net = __toESM(require("node:net"));
    path11 = __toESM(require("node:path"));
    os = __toESM(require("node:os"));
    fs = __toESM(require("node:fs"));
    import_node_child_process7 = require("node:child_process");
    init_pty_daemon_protocol();
    REQUEST_TIMEOUT_MS = 15e3;
    CONNECT_TIMEOUT_MS = 2e3;
    SPAWN_WAIT_TIMEOUT_MS = 5e3;
    SPAWN_WAIT_POLL_MS = 100;
    socket = null;
    connectingPromise = null;
    nextRequestId = 1;
    pendingRequests = /* @__PURE__ */ new Map();
    currentNotify = null;
  }
});

// src/relay/pty-agent-bridge.ts
function resumeFrom2(params) {
  const t = params["_trace"];
  if (t && typeof t === "object" && typeof t.id === "string") {
    return { id: t.id };
  }
  return void 0;
}
function attachIdentityMismatches(expected, entry) {
  return Boolean(
    expected.paneKey && entry.paneKey && expected.paneKey !== entry.paneKey || expected.tabId && entry.tabId && expected.tabId !== entry.tabId
  );
}
function safeCwd(raw) {
  if (!raw) {return require("node:os").homedir();}
  if (raw.includes("\0")) {return require("node:os").homedir();}
  const resolved = require("node:path").resolve(raw);
  try {
    const stat5 = require("node:fs").statSync(resolved);
    if (!stat5.isDirectory()) {return require("node:os").homedir();}
    return resolved;
  } catch {
    return require("node:os").homedir();
  }
}
function appendScrollback(entry, data) {
  entry.buf += data;
  const lines = entry.buf.split("\n");
  if (lines.length > SCROLLBACK_LINES) {
    entry.buf = lines.slice(lines.length - SCROLLBACK_LINES).join("\n");
  }
}
async function handlePtyCreate2(id, params, log, notify) {
  const cols = typeof params.cols === "number" ? params.cols : 80;
  const rows = typeof params.rows === "number" ? params.rows : 24;
  const rawCwd = typeof params.cwd === "string" ? params.cwd : "";
  const span = Tracers.terminalCreate.start({ origin: "agent-pty", cols, rows }, resumeFrom2(params));
  const nodePty = await import("node-pty").catch(() => null);
  if (!nodePty) {
    span.fail("node-pty not available on this host");
    return {
      jsonrpc: "2.0",
      id,
      error: { code: -32603, message: "node-pty is not available on this host" }
    };
  }
  const cwd = safeCwd(rawCwd);
  const envOverride = params.env && typeof params.env === "object" && !Array.isArray(params.env) ? params.env : {};
  const shellOverride = typeof params.shellOverride === "string" ? params.shellOverride.trim() : "";
  const envShell = typeof envOverride.SHELL === "string" ? envOverride.SHELL.trim() : "";
  const shell = shellOverride || envShell || (process.env.SHELL ?? "/bin/sh");
  const ptyId = `agent-pty-${nextAgentPtyId++}`;
  const paneKey = typeof params.paneKey === "string" ? params.paneKey : void 0;
  const tabId = typeof params.tabId === "string" ? params.tabId : void 0;
  try {
    span.step("node-pty-spawn", { shell, cwd });
    const term = nodePty.spawn(shell, [], {
      name: "xterm-256color",
      cols,
      rows,
      cwd,
      env: { ...process.env, TERM: "xterm-256color", ...envOverride }
    });
    const entry = { pty: term, cwd, cols, rows, shell, buf: "", paneKey, tabId, notify };
    AGENT_PTY_MAP.set(ptyId, entry);
    term.onData((data) => {
      appendScrollback(entry, data);
      entry.notify("pty.data", { id: ptyId, data });
    });
    term.onExit(({ exitCode, signal }) => {
      if (entry.graceTimer) {clearTimeout(entry.graceTimer);}
      AGENT_PTY_MAP.delete(ptyId);
      entry.notify("pty.exit", { id: ptyId, exitCode, signal: signal ?? null });
    });
    log.info(`pty.create (agent): id=${ptyId} cwd=${cwd} shell=${shell}`);
    span.ok({ ptyId, shell, cwd });
    return { jsonrpc: "2.0", id, result: { id: ptyId, cols, rows, cwd, shell } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    log.error(`pty.create (agent): failed: ${msg}`);
    span.fail(err, { cwd, shell });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `pty.create failed: ${msg}` } };
  }
}
async function handlePtyAttach2(id, params, log, notify) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const span = Tracers.terminalReattach.start({ origin: "agent-pty", ptyId }, resumeFrom2(params));
  if (!ptyId) {
    span.fail("missing id");
    return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.attach: missing id" } };
  }
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {
    span.fail("pty not found", { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY "${ptyId}" not found` } };
  }
  const expectedPaneKey = typeof params.expectedPaneKey === "string" ? params.expectedPaneKey : void 0;
  const expectedTabId = typeof params.expectedTabId === "string" ? params.expectedTabId : void 0;
  if (attachIdentityMismatches({ paneKey: expectedPaneKey, tabId: expectedTabId }, entry)) {
    span.fail("identity mismatch", { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY "${ptyId}" not found (identity mismatch)` } };
  }
  entry.notify = notify;
  const wasWithinGracePeriod = Boolean(entry.graceTimer);
  if (entry.graceTimer) {
    clearTimeout(entry.graceTimer);
    entry.graceTimer = null;
  }
  const cols = typeof params.cols === "number" ? params.cols : void 0;
  const rows = typeof params.rows === "number" ? params.rows : void 0;
  if (cols && rows) {
    try {
      entry.pty.resize(cols, rows);
      entry.cols = cols;
      entry.rows = rows;
    } catch {
    }
  }
  log.info(`pty.attach (agent): id=${ptyId}`);
  span.ok({ ptyId, wasWithinGracePeriod, replayBytes: entry.buf.length });
  if (params.suppressReplayNotification) {
    return { jsonrpc: "2.0", id, result: { replay: entry.buf } };
  }
  if (entry.buf) {
    notify("pty.replay", { id: ptyId, data: entry.buf });
  }
  return { jsonrpc: "2.0", id, result: {} };
}
function scheduleGracePeriodCleanup(log, graceTimeMs = PTY_GRACE_PERIOD_MS) {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    if (entry.graceTimer) {continue;}
    entry.graceTimer = setTimeout(() => {
      const current = AGENT_PTY_MAP.get(ptyId);
      if (!current || current.graceTimer !== entry.graceTimer) {return;}
      try {
        current.pty.kill("SIGTERM");
        log.info(`scheduleGracePeriodCleanup: grace period expired, killed ${ptyId}`);
      } catch {
      }
      AGENT_PTY_MAP.delete(ptyId);
    }, graceTimeMs);
  }
  if (AGENT_PTY_MAP.size > 0) {
    log.info(`scheduleGracePeriodCleanup: armed grace timers for ${AGENT_PTY_MAP.size} PTY(s) (${graceTimeMs}ms)`);
  }
}
async function handlePtyWrite2(id, params, _log) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const data = typeof params.data === "string" ? params.data : "";
  if (!ptyId) {return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.write: missing id" } };}
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY not found: ${ptyId}` } };}
  try {
    entry.pty.write(data);
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `pty.write failed: ${msg}` } };
  }
}
async function handlePtyResize2(id, params, _log) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const cols = typeof params.cols === "number" ? params.cols : 80;
  const rows = typeof params.rows === "number" ? params.rows : 24;
  const span = Tracers.terminalResize.start({ origin: "agent-pty", ptyId, cols, rows }, resumeFrom2(params));
  if (!ptyId) {
    span.fail("missing id");
    return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.resize: missing id" } };
  }
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {
    span.fail("pty not found", { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY not found: ${ptyId}` } };
  }
  try {
    entry.pty.resize(cols, rows);
    entry.cols = cols;
    entry.rows = rows;
    span.ok({ ptyId, cols, rows });
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `pty.resize failed: ${msg}` } };
  }
}
async function handlePtyDestroy2(id, params, log) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const graceful = params.graceful !== false;
  const span = Tracers.terminalDestroy.start({ origin: "agent-pty", ptyId, graceful }, resumeFrom2(params));
  if (!ptyId) {
    span.fail("missing id");
    return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.destroy: missing id" } };
  }
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {
    span.ok({ ptyId, alreadyDead: true });
    return { jsonrpc: "2.0", id, result: { ok: true, alreadyDead: true } };
  }
  try {
    if (entry.graceTimer) {clearTimeout(entry.graceTimer);}
    if (process.platform === "win32") {
      entry.pty.kill();
    } else {
      entry.pty.kill(graceful ? "SIGTERM" : "SIGKILL");
    }
    AGENT_PTY_MAP.delete(ptyId);
    log.info(`pty.destroy (agent): id=${ptyId}`);
    span.ok({ ptyId, graceful });
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span.fail(err, { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `pty.destroy failed: ${msg}` } };
  }
}
async function handlePtyScrollback2(id, params, _log) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const lines = typeof params.lines === "number" ? params.lines : 100;
  if (!ptyId) {return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.scrollback: missing id" } };}
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY not found: ${ptyId}` } };}
  const allLines = entry.buf.split("\n");
  const data = allLines.slice(Math.max(0, allLines.length - lines)).join("\n");
  return { jsonrpc: "2.0", id, result: { data } };
}
async function handlePtySendSignal2(id, params, log) {
  const ptyId = typeof params.id === "string" ? params.id : "";
  const signal = typeof params.signal === "string" ? params.signal : "SIGTERM";
  const isTerminating = signal === "SIGKILL" || signal === "SIGTERM";
  const span = isTerminating ? Tracers.terminalDestroy.start({ origin: "agent-pty", ptyId, signal, via: "pty.sendSignal" }, resumeFrom2(params)) : void 0;
  if (!ptyId) {
    span?.fail("missing id");
    return { jsonrpc: "2.0", id, error: { code: -32602, message: "pty.sendSignal: missing id" } };
  }
  if (!ALLOWED_SIGNALS.has(signal)) {
    span?.fail(`signal not allowed: ${signal}`);
    return { jsonrpc: "2.0", id, error: { code: -32602, message: `Signal not allowed: ${signal}` } };
  }
  const entry = AGENT_PTY_MAP.get(ptyId);
  if (!entry) {
    span?.fail("pty not found", { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `PTY not found: ${ptyId}` } };
  }
  try {
    if (process.platform !== "win32") {
      entry.pty.kill(signal);
    } else {
      entry.pty.kill();
    }
    log.info(`pty.sendSignal (agent): id=${ptyId} signal=${signal}`);
    span?.ok({ ptyId, signal });
    return { jsonrpc: "2.0", id, result: { ok: true } };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    span?.fail(err, { ptyId });
    return { jsonrpc: "2.0", id, error: { code: -32603, message: `pty.sendSignal failed: ${msg}` } };
  }
}
function activePtyCount() {
  return AGENT_PTY_MAP.size;
}
function cleanupAgentPtys(log) {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    try {
      if (entry.graceTimer) {clearTimeout(entry.graceTimer);}
      entry.pty.kill("SIGTERM");
      log.info(`cleanupAgentPtys: killed ${ptyId}`);
    } catch {
    }
  }
  AGENT_PTY_MAP.clear();
}
var AGENT_PTY_MAP, nextAgentPtyId, ALLOWED_SIGNALS, SCROLLBACK_LINES, PTY_GRACE_PERIOD_MS;
var init_pty_agent_bridge = __esm({
  "src/relay/pty-agent-bridge.ts"() {
    "use strict";
    init_tracers();
    AGENT_PTY_MAP = /* @__PURE__ */ new Map();
    nextAgentPtyId = 1;
    ALLOWED_SIGNALS = /* @__PURE__ */ new Set(["SIGTERM", "SIGKILL", "SIGINT", "SIGHUP", "SIGTSTP"]);
    SCROLLBACK_LINES = 500;
    PTY_GRACE_PERIOD_MS = 12e4;
  }
});

// src/relay/pty-daemon-server.ts
var pty_daemon_server_exports = {};
__export(pty_daemon_server_exports, {
  runPtyDaemon: () => runPtyDaemon
});
function toDaemonOutcome(handlerResult) {
  const shaped = handlerResult;
  if (shaped.error) {
    return { error: { message: typeof shaped.error.message === "string" ? shaped.error.message : "Unknown error" } };
  }
  return { result: shaped.result };
}
async function dispatchDaemonRequest(method, params, log, notify) {
  switch (method) {
    case "pty.create":
      return toDaemonOutcome(await handlePtyCreate2(null, params, log, notify));
    case "pty.attach":
      return toDaemonOutcome(await handlePtyAttach2(null, params, log, notify));
    case "pty.write":
      return toDaemonOutcome(await handlePtyWrite2(null, params, log));
    case "pty.resize":
      return toDaemonOutcome(await handlePtyResize2(null, params, log));
    case "pty.destroy":
      return toDaemonOutcome(await handlePtyDestroy2(null, params, log));
    case "pty.scrollback":
      return toDaemonOutcome(await handlePtyScrollback2(null, params, log));
    case "pty.sendSignal":
      return toDaemonOutcome(await handlePtySendSignal2(null, params, log));
    case "daemon.ping":
      return { result: { ok: true, ptys: activePtyCount() } };
    case "daemon.sessionClosed":
      scheduleGracePeriodCleanup(log);
      return { result: { ok: true } };
    default:
      return { error: { message: `Unknown daemon method: ${method}` } };
  }
}
function probeExistingDaemon(socketPath) {
  return new Promise((resolve8) => {
    const sock = net2.createConnection(socketPath);
    const timer = setTimeout(() => {
      sock.destroy();
      resolve8(false);
    }, PROBE_TIMEOUT_MS);
    sock.once("connect", () => {
      clearTimeout(timer);
      sock.end();
      resolve8(true);
    });
    sock.once("error", () => {
      clearTimeout(timer);
      resolve8(false);
    });
  });
}
async function runPtyDaemon(socketPath, log) {
  if (await probeExistingDaemon(socketPath)) {
    log.info(`pty-daemon: another instance is already listening on ${socketPath} \u2014 exiting`);
    process.exit(0);
  }
  try {
    fs2.unlinkSync(socketPath);
  } catch {
  }
  const clients = /* @__PURE__ */ new Set();
  let idleTimer = null;
  const broadcast = (method, params) => {
    const line = encodeDaemonMessage({ method, params });
    for (const client of clients) {
      client.write(line);
    }
  };
  const armIdleShutdownIfEmpty = () => {
    if (idleTimer) {clearTimeout(idleTimer);}
    idleTimer = null;
    if (clients.size > 0 || activePtyCount() > 0) {return;}
    idleTimer = setTimeout(() => {
      if (clients.size === 0 && activePtyCount() === 0) {
        log.info("pty-daemon: idle with no PTYs and no clients \u2014 shutting down");
        server.close(() => process.exit(0));
      }
    }, PTY_DAEMON_IDLE_SHUTDOWN_MS);
  };
  const server = net2.createServer((socket2) => {
    clients.add(socket2);
    if (idleTimer) {
      clearTimeout(idleTimer);
      idleTimer = null;
    }
    const decoder = new DaemonMessageDecoder((msg) => {
      if (!isDaemonRequest(msg)) {return;}
      void dispatchDaemonRequest(msg.method, msg.params ?? {}, log, broadcast).then((outcome) => {
        const response = { id: msg.id, ...outcome };
        socket2.write(encodeDaemonMessage(response));
      }).catch((err) => {
        const message = err instanceof Error ? err.message : String(err);
        socket2.write(encodeDaemonMessage({ id: msg.id, error: { message } }));
      });
    });
    socket2.on("data", (chunk) => decoder.feed(chunk.toString("utf8")));
    socket2.on("close", () => {
      clients.delete(socket2);
      armIdleShutdownIfEmpty();
    });
    socket2.on("error", () => {
      clients.delete(socket2);
      armIdleShutdownIfEmpty();
    });
  });
  server.listen(socketPath, () => {
    log.info(`pty-daemon: listening on ${socketPath} (pid=${process.pid})`);
  });
  const shutdown = (signal) => {
    log.info(`pty-daemon: shutting down (${signal})`);
    cleanupAgentPtys(log);
    server.close(() => process.exit(0));
  };
  process.on("SIGTERM", () => shutdown("SIGTERM"));
  process.on("SIGINT", () => shutdown("SIGINT"));
  armIdleShutdownIfEmpty();
}
var net2, fs2, PTY_DAEMON_IDLE_SHUTDOWN_MS, PROBE_TIMEOUT_MS;
var init_pty_daemon_server = __esm({
  "src/relay/pty-daemon-server.ts"() {
    "use strict";
    net2 = __toESM(require("node:net"));
    fs2 = __toESM(require("node:fs"));
    init_pty_agent_bridge();
    init_pty_daemon_protocol();
    PTY_DAEMON_IDLE_SHUTDOWN_MS = 10 * 6e4;
    PROBE_TIMEOUT_MS = 1500;
  }
});

// src/relay/agent-config.ts
var import_node_os = require("node:os");
var import_node_path = require("node:path");
function buildToolPath(home) {
  return [
    `${home}/.local/bin`,
    `${home}/bin`,
    "/usr/local/bin",
    "/usr/bin",
    "/bin",
    "/usr/sbin",
    "/snap/bin"
  ].join(":");
}
function deriveHttpUrl(wsUrl) {
  try {
    const u = new URL(wsUrl);
    u.protocol = u.protocol === "wss:" ? "https:" : "http:";
    u.pathname = "/";
    u.search = "";
    return u.origin;
  } catch {
    return "http://localhost:6769";
  }
}
function loadAgentConfig() {
  const rawMode = process.env.MODE || "direct-websocket";
  if (rawMode !== "direct-websocket" && rawMode !== "relay-websocket") {
    throw new Error(
      `Invalid MODE="${rawMode}". Must be "direct-websocket" or "relay-websocket".`
    );
  }
  const mode = rawMode;
  const home = (0, import_node_os.homedir)();
  const toolPath = buildToolPath(home);
  const orcaUrl = process.env.ORCA_URL || "wss://b15.openledger.vn/agent";
  const orcaHttpUrl = process.env.ORCA_HTTP_URL || deriveHttpUrl(orcaUrl);
  return {
    mode,
    orcaUrl,
    orcaHttpUrl,
    agentToken: process.env.AGENT_TOKEN ?? "",
    apiSecret: process.env.ORCA_AGENT_API_SECRET ?? "",
    agentPort: Number.parseInt(process.env.AGENT_PORT || "6799", 10),
    devServerId: process.env.DEV_SERVER_ID || "dev-local",
    logLevel: process.env.LOG_LEVEL || "info",
    workDir: process.env.AGENT_WORK_DIR || process.cwd(),
    toolPath,
    toolEnv: {
      ...process.env,
      PATH: toolPath,
      HOME: home,
      ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY ?? "",
      GITHUB_TOKEN: process.env.GITHUB_TOKEN ?? "",
      GH_TOKEN: process.env.GH_TOKEN ?? ""
    },
    credentialDir: (0, import_node_path.join)(home, ".orca", "credentials"),
    tlsRejectUnauthorized: process.env.NODE_TLS_REJECT_UNAUTHORIZED !== "0"
  };
}

// src/relay/agent-logger.ts
function createAgentLogger(level) {
  const ts = () => (/* @__PURE__ */ new Date()).toISOString();
  return {
    info: (...a) => console.log(`[agent] ${ts()} INFO  ${a.join(" ")}`),
    warn: (...a) => console.warn(`[agent] ${ts()} WARN  ${a.join(" ")}`),
    error: (...a) => console.error(`[agent] ${ts()} ERROR ${a.join(" ")}`),
    debug: (...a) => {
      if (level === "debug") {console.log(`[agent] ${ts()} DEBUG ${a.join(" ")}`);}
    }
  };
}

// src/relay/agent-tool-registry.ts
var import_node_child_process = require("node:child_process");
var import_node_fs = require("node:fs");
var import_promises = require("node:fs/promises");
var import_node_path2 = require("node:path");
function resolveToolBinary(binary, toolPath) {
  for (const dir of toolPath.split(":")) {
    if (!dir) {continue;}
    const candidate = (0, import_node_path2.join)(dir, binary);
    try {
      (0, import_node_fs.accessSync)(candidate, import_node_fs.constants.X_OK);
      return candidate;
    } catch {
    }
  }
  return binary;
}
function runToolCommand(binary, args, opts) {
  return new Promise((resolve8) => {
    const resolved = resolveToolBinary(binary, opts.env.PATH ?? "");
    const child = (0, import_node_child_process.spawn)(resolved, args, {
      cwd: opts.cwd,
      env: opts.env,
      stdio: ["pipe", "pipe", "pipe"],
      shell: false
      // CRITICAL: no shell — prevents injection attacks
    });
    const stdout = [];
    const stderr = [];
    const timer = setTimeout(() => {
      child.kill("SIGTERM");
      resolve8({
        stdout: stdout.join(""),
        stderr: `${stderr.join("")}
[TIMEOUT: command exceeded ${opts.timeout}ms limit]`,
        exitCode: 124
      });
    }, opts.timeout);
    child.stdout?.on("data", (chunk) => stdout.push(chunk.toString()));
    child.stderr?.on("data", (chunk) => stderr.push(chunk.toString()));
    child.on("close", (code) => {
      clearTimeout(timer);
      resolve8({ stdout: stdout.join(""), stderr: stderr.join(""), exitCode: code ?? 0 });
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      resolve8({ stdout: "", stderr: err.message, exitCode: 1 });
    });
    child.stdin?.end();
  });
}
var ALL_TOOL_DEFINITIONS = [
  // ── claude_code ──────────────────────────────────────────────────────────
  {
    name: "claude_code",
    binary: "claude",
    description: "Run Claude Code AI assistant in --print mode (non-interactive). Streams output when done.",
    inputSchema: {
      type: "object",
      properties: {
        prompt: { type: "string", description: "Task or question for Claude Code to execute" },
        cwd: { type: "string", description: "Working directory (absolute). Defaults to AGENT_WORK_DIR." },
        model: { type: "string", description: "Claude model override (e.g. claude-opus-4-5). Empty = CLI default.", default: "" }
      },
      required: ["prompt"]
    },
    async handler(params, config) {
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      const args = ["--print", String(params.prompt)];
      if (typeof params.model === "string" && params.model) {
        args.unshift("--model", params.model);
      }
      return runToolCommand("claude", args, { cwd, timeout: 3e5, env: config.toolEnv });
    }
  },
  // ── gh ───────────────────────────────────────────────────────────────────
  {
    name: "gh",
    binary: "gh",
    description: "GitHub CLI \u2014 manage PRs, issues, repos, gists, and more.",
    inputSchema: {
      type: "object",
      properties: {
        args: { type: "array", items: { type: "string" }, description: 'gh subcommand and arguments (e.g. ["pr", "list"])' },
        cwd: { type: "string", description: "Working directory (git repository)" }
      },
      required: ["args"]
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : [];
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      return runToolCommand("gh", args, { cwd, timeout: 6e4, env: config.toolEnv });
    }
  },
  // ── git ──────────────────────────────────────────────────────────────────
  {
    name: "git",
    binary: "git",
    description: "Run git commands on the dev server. For UI-driven operations prefer the git.exec RPC method (has whitelist validation).",
    inputSchema: {
      type: "object",
      properties: {
        args: { type: "array", items: { type: "string" }, description: 'git arguments (e.g. ["log", "--oneline", "-10"])' },
        cwd: { type: "string", description: "Working directory (git repository root)" }
      },
      required: ["args"]
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : [];
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      return runToolCommand("git", args, { cwd, timeout: 6e4, env: config.toolEnv });
    }
  },
  // ── gitnexus ─────────────────────────────────────────────────────────────
  {
    name: "gitnexus",
    binary: "gitnexus",
    description: "GitNexus code intelligence CLI. Query the codebase graph, find symbols and their usages.",
    inputSchema: {
      type: "object",
      properties: {
        args: { type: "array", items: { type: "string" }, description: "gitnexus subcommand and arguments" },
        cwd: { type: "string", description: "Working directory (project with .codegraph/ or gitnexus index)" }
      },
      required: ["args"]
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : [];
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      return runToolCommand("gitnexus", args, { cwd, timeout: 6e4, env: config.toolEnv });
    }
  },
  // ── codegraph ────────────────────────────────────────────────────────────
  {
    name: "codegraph",
    binary: "codegraph",
    description: "CodeGraph local code analysis. Explore symbols, dependencies, and call graphs.",
    inputSchema: {
      type: "object",
      properties: {
        args: { type: "array", items: { type: "string" }, description: "codegraph subcommand and arguments" },
        cwd: { type: "string", description: "Working directory (project with .codegraph/)" }
      },
      required: ["args"]
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : [];
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      return runToolCommand("codegraph", args, { cwd, timeout: 6e4, env: config.toolEnv });
    }
  },
  // ── docker ───────────────────────────────────────────────────────────────
  {
    name: "docker",
    binary: "docker",
    description: "Run Docker CLI commands. Inspect containers, images, volumes, and logs.",
    inputSchema: {
      type: "object",
      properties: {
        args: { type: "array", items: { type: "string" }, description: 'docker subcommand and arguments (e.g. ["ps", "-a"])' },
        cwd: { type: "string", description: "Working directory" }
      },
      required: ["args"]
    },
    async handler(params, config) {
      const args = Array.isArray(params.args) ? params.args.map(String) : [];
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      return runToolCommand("docker", args, { cwd, timeout: 12e4, env: config.toolEnv });
    }
  },
  // ── shell ────────────────────────────────────────────────────────────────
  {
    name: "shell",
    binary: "bash",
    description: "Run an arbitrary shell command on the dev server via bash -c. Output is captured and returned.",
    inputSchema: {
      type: "object",
      properties: {
        command: { type: "string", description: 'Shell command to execute (runs as: bash -c "<command>")' },
        cwd: { type: "string", description: "Working directory" },
        timeout: { type: "number", description: "Timeout in milliseconds (default: 60000, max: 600000)", default: 6e4 }
      },
      required: ["command"]
    },
    async handler(params, config) {
      const cwd = typeof params.cwd === "string" && params.cwd ? params.cwd : config.workDir;
      const timeout = Math.min(typeof params.timeout === "number" ? params.timeout : 6e4, 6e5);
      return runToolCommand("bash", ["-c", String(params.command)], { cwd, timeout, env: config.toolEnv });
    }
  },
  // ── read_file (built-in) ─────────────────────────────────────────────────
  {
    name: "read_file",
    binary: null,
    // built-in: always available, no binary check
    description: "Read a file from the dev server filesystem. Supports line range selection.",
    inputSchema: {
      type: "object",
      properties: {
        path: { type: "string", description: "File path (absolute, or relative to AGENT_WORK_DIR)" },
        start_line: { type: "number", description: "First line to read, 1-indexed (default: 1)", default: 1 },
        end_line: { type: "number", description: "Last line to read, inclusive (default: end of file)" }
      },
      required: ["path"]
    },
    async handler(params, config) {
      const filePath = typeof params.path === "string" && (0, import_node_path2.isAbsolute)(params.path) ? params.path : (0, import_node_path2.join)(config.workDir, String(params.path ?? ""));
      try {
        const content = await (0, import_promises.readFile)(filePath, "utf8");
        const lines = content.split("\n");
        const start = Math.max(0, (typeof params.start_line === "number" ? params.start_line : 1) - 1);
        const end = typeof params.end_line === "number" ? params.end_line : lines.length;
        const slice = lines.slice(start, end).join("\n");
        return {
          stdout: slice,
          stderr: "",
          exitCode: 0,
          meta: { path: filePath, totalLines: lines.length, startLine: start + 1, endLine: Math.min(end, lines.length) }
        };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return { stdout: "", stderr: msg, exitCode: 1 };
      }
    }
  },
  // ── list_dir (built-in) ──────────────────────────────────────────────────
  {
    name: "list_dir",
    binary: null,
    // built-in: always available, no binary check
    description: "List directory contents on the dev server. Returns name, type, and size for each entry.",
    inputSchema: {
      type: "object",
      properties: {
        path: { type: "string", description: "Directory path (absolute, or relative to AGENT_WORK_DIR)" }
      },
      required: ["path"]
    },
    async handler(params, config) {
      const dirPath = typeof params.path === "string" && (0, import_node_path2.isAbsolute)(params.path) ? params.path : (0, import_node_path2.join)(config.workDir, String(params.path ?? ""));
      try {
        const entries = await (0, import_promises.readdir)(dirPath, { withFileTypes: true });
        const list = await Promise.all(
          entries.map(async (e) => ({
            name: e.name,
            type: e.isDirectory() ? "dir" : "file",
            size: e.isFile() ? (await (0, import_promises.stat)((0, import_node_path2.join)(dirPath, e.name))).size : null
          }))
        );
        return { stdout: JSON.stringify(list, null, 2), stderr: "", exitCode: 0 };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return { stdout: "", stderr: msg, exitCode: 1 };
      }
    }
  }
];
async function discoverTools(config) {
  const pathDirs = config.toolPath.split(":").filter(Boolean);
  const discovered = [];
  for (const tool of ALL_TOOL_DEFINITIONS) {
    if (tool.binary === null) {
      discovered.push(tool);
      continue;
    }
    const found = pathDirs.some((dir) => {
      try {
        (0, import_node_fs.accessSync)((0, import_node_path2.join)(dir, tool.binary), import_node_fs.constants.X_OK);
        return true;
      } catch {
        return false;
      }
    });
    if (found) {discovered.push(tool);}
  }
  return discovered;
}

// ../node_modules/.pnpm/ws@8.21.0/node_modules/ws/wrapper.mjs
var import_stream = __toESM(require_stream(), 1);
var import_extension = __toESM(require_extension(), 1);
var import_permessage_deflate = __toESM(require_permessage_deflate(), 1);
var import_receiver = __toESM(require_receiver(), 1);
var import_sender = __toESM(require_sender(), 1);
var import_subprotocol = __toESM(require_subprotocol(), 1);
var import_websocket = __toESM(require_websocket(), 1);
var import_websocket_server = __toESM(require_websocket_server(), 1);
var wrapper_default = import_websocket.default;

// src/relay/agent-session.ts
init_agent_wire();

// src/relay/agent-rpc-dispatch.ts
init_agent_wire();
init_agent_wire_protocol();
init_trace();
init_tracers();
var rpcTracer = createTracer("agent:rpc");
function extractResume2(params) {
  const t = params["_trace"];
  if (t && typeof t === "object" && typeof t.id === "string") {
    return { id: t.id };
  }
  return void 0;
}
function extractTraceFields(method, params) {
  const p = params;
  const str = (v) => typeof v === "string" ? v : void 0;
  const num = (v) => typeof v === "number" ? v : void 0;
  const truncPath = (v) => {
    const s = str(v);
    return s ? s.length > 60 ? `...${  s.slice(-57)}` : s : void 0;
  };
  const truncCmd = (v) => {
    const s = str(v);
    return s ? s.length > 80 ? `${s.slice(0, 77)  }...` : s : void 0;
  };
  if (method.startsWith("fs.") || method === "shell.eval" || method === "preflight.check") {
    return {
      path: truncPath(p["path"] ?? p["dir"] ?? p["filePath"]),
      pattern: str(p["pattern"]),
      cmd: method === "shell.eval" ? truncCmd(p["cmd"]) : void 0
    };
  }
  if (method.startsWith("git.")) {
    return {
      repo: truncPath(p["repoPath"] ?? p["workDir"]),
      cmd: truncCmd(p["cmd"] ?? p["args"]),
      branch: str(p["branch"]),
      worktree: truncPath(p["worktreePath"] ?? p["path"])
    };
  }
  if (method.startsWith("github.") || method.startsWith("gitlab.")) {
    return {
      repo: str(p["repo"] ?? p["project"]),
      branch: str(p["branch"] ?? p["sourceBranch"]),
      prNum: num(p["prNumber"] ?? p["mrIid"]),
      title: str(p["title"])?.slice(0, 40)
    };
  }
  if (method.startsWith("ai.provider.")) {
    return {
      provider: str(p["provider"] ?? p["providerId"])
      // Never log credential value — just the provider name
    };
  }
  if (method === "tools/call") {
    return {
      tool: str(p["name"])
    };
  }
  if (method === "ai.complete") {
    return {
      model: str(p["model"]),
      taskId: str(p["taskId"]),
      promptLength: typeof p["prompt"] === "string" ? p["prompt"].length : void 0
    };
  }
  if (method === "agent.exec") {
    return {
      // (TASK-AG-015.1) base — request shape:
      binary: str(p["binary"]),
      argsCount: Array.isArray(p["args"]) ? p["args"].length : void 0,
      hasEnvOverride: p["env"] !== void 0 && p["env"] !== null,
      timeoutMs: num(p["timeoutMs"]),
      // CR-TRACE-017 BL-WF-02: StepExecutors.executeAgent() already sends
      // `stepId` — this field has a real value immediately, no backend change needed.
      stepId: str(p["stepId"]),
      // CR-TRACE-017 §4: `parentTraceId` is a plain business field so the
      // TracePanel can group every step-span of the same workflow execution —
      // NOT Tracer.start()'s `resume` mechanism (CR-TRACE-000 §3.1), since that
      // core API hasn't shipped. Only populated once WorkflowOrchestrator.ts is
      // updated to send `traceId: stepSpan.id` + `parentTraceId: rootTraceId` in
      // the relay.call('agent.exec', ...) params — until then this stays
      // undefined without error (agent side is ready, no second edit needed).
      parentTraceId: str(p["parentTraceId"]),
      // CR-TRACE-018 BL-TG-04: only populated once the backend
      // (ProfileAwareAgentSpawner.spawn() / TaskAgentExecutor) is updated to
      // send `taskId` as a top-level param instead of only inside
      // `env.ORCA_TASK_ID` — until then this stays undefined without error.
      taskId: str(p["taskId"])
    };
  }
  if (method.startsWith("agent.")) {
    return {
      session: str(p["sessionId"] ?? p["taskId"]),
      binary: str(p["binary"] ?? p["model"] ?? p["modelId"]),
      cmd: truncCmd(p["cmd"] ?? p["command"])
    };
  }
  return {};
}
function extractResultFields(method, result) {
  if (method === "agent.exec" && result && typeof result === "object") {
    const r = result;
    return {
      exitCode: typeof r["exitCode"] === "number" ? r["exitCode"] : void 0,
      timedOut: typeof r["timedOut"] === "boolean" ? r["timedOut"] : void 0
    };
  }
  return {};
}
function createRpcDispatcher(tools, config, log) {
  return {
    async dispatch(ws, state, rpc) {
      const ctxFields = extractTraceFields(rpc.method, rpc.params ?? {});
      const span = rpcTracer.start(
        { method: rpc.method, id: String(rpc.id ?? "notify"), ...ctxFields },
        extractResume2(rpc.params ?? {})
      );
      let response;
      try {
        response = await route(rpc, tools, config, log, ws, state);
        if ("error" in response) {
          span.fail(response.error.message, { method: rpc.method, code: response.error.code });
        } else {
          span.ok({ method: rpc.method, ...extractResultFields(rpc.method, response.result) });
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        log.error(`RPC dispatch unhandled error method=${rpc.method}: ${msg}`);
        span.fail(msg, { method: rpc.method, phase: "dispatch" });
        response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`);
      }
      if (ws.readyState === 1) {
        ws.send(encodeDataFrame(state, JSON.stringify(response)));
      }
    }
  };
}
function makeNotifier(ws, state) {
  return (method, params) => {
    if (ws.readyState !== 1) {return;}
    ws.send(encodeDataFrame(state, JSON.stringify({ jsonrpc: "2.0", method, params })));
  };
}
async function route(rpc, tools, config, log, ws, state) {
  switch (rpc.method) {
    // ── MCP: tools/list ──────────────────────────────────────────────────────
    case "tools/list":
      return {
        jsonrpc: "2.0",
        id: rpc.id,
        result: {
          tools: tools.map((t) => ({
            name: t.name,
            description: t.description,
            inputSchema: t.inputSchema
          }))
        }
      };
    // ── MCP: tools/call ──────────────────────────────────────────────────────
    case "tools/call": {
      const params = rpc.params ?? {};
      const name = typeof params.name === "string" ? params.name : "";
      const args = typeof params.arguments === "object" && params.arguments !== null ? params.arguments : {};
      const tool = tools.find((t) => t.name === name);
      if (!tool) {
        return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Tool not found: ${name}`);
      }
      log.info(`tools/call name=${name} args=${JSON.stringify(args).slice(0, 120)}`);
      let result;
      try {
        result = await tool.handler(args, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        log.error(`tools/call handler threw name=${name}: ${msg}`);
        return makeError(rpc.id, AgentErrorCode.ServerError, `Tool handler error: ${msg}`);
      }
      return formatMcpResult(rpc.id, result);
    }
    // ── v5.0: git.exec ───────────────────────────────────────────────────────
    case "git.exec": {
      try {
        const { handleGitExec: handleGitExec2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        return await handleGitExec2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.exec unavailable: ${msg}`);
      }
    }
    // ── v5.0: git.execStream ─────────────────────────────────────────────────
    case "git.execStream": {
      try {
        const { handleGitExecStream: handleGitExecStream2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        void handleGitExecStream2(ws, state, rpc.id, rpc.params ?? {}, config, log);
        return { jsonrpc: "2.0", id: rpc.id, result: { type: "stream.started" } };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.execStream unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.readDir ─────────────────────────────────────────────────────
    case "fs.readDir": {
      try {
        const { handleFsReadDir: handleFsReadDir2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsReadDir2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readDir unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.readFile ────────────────────────────────────────────────────
    case "fs.readFile": {
      try {
        const { handleFsReadFile: handleFsReadFile2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsReadFile2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readFile unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.grep ────────────────────────────────────────────────────────
    case "fs.grep": {
      try {
        const { handleFsGrep: handleFsGrep2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsGrep2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.grep unavailable: ${msg}`);
      }
    }
    // ── v5.0: ai.provider.writeCredential ────────────────────────────────────
    case "ai.provider.writeCredential": {
      try {
        const { handleWriteCredential: handleWriteCredential2 } = await Promise.resolve().then(() => (init_agent_credential_store(), agent_credential_store_exports));
        return await handleWriteCredential2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.writeCredential unavailable: ${msg}`);
      }
    }
    // ── v5.0: ai.provider.readCredential ─────────────────────────────────────
    case "ai.provider.readCredential": {
      try {
        const { handleReadCredential: handleReadCredential2 } = await Promise.resolve().then(() => (init_agent_credential_store(), agent_credential_store_exports));
        return await handleReadCredential2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.readCredential unavailable: ${msg}`);
      }
    }
    // ── v5.0: ai.provider.healthCheck ────────────────────────────────────────
    case "ai.provider.healthCheck": {
      try {
        const { handleHealthCheck: handleHealthCheck2 } = await Promise.resolve().then(() => (init_agent_credential_store(), agent_credential_store_exports));
        return await handleHealthCheck2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.healthCheck unavailable: ${msg}`);
      }
    }
    // ── v5.0: preflight.check ────────────────────────────────────────────────
    case "preflight.check": {
      try {
        const { handlePreflightCheck: handlePreflightCheck2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handlePreflightCheck2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `preflight.check unavailable: ${msg}`);
      }
    }
    // ── v5.0: ai.provider.deleteCredential ───────────────────────────────────
    case "ai.provider.deleteCredential": {
      try {
        const { handleDeleteCredential: handleDeleteCredential2 } = await Promise.resolve().then(() => (init_agent_credential_store(), agent_credential_store_exports));
        return await handleDeleteCredential2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.deleteCredential unavailable: ${msg}`);
      }
    }
    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case "git.pr.create": {
      try {
        const { handleGitPrCreate: handleGitPrCreate2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        return await handleGitPrCreate2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`);
      }
    }
    // ── v5.0: git.worktree.list ──────────────────────────────────────────────
    case "git.worktree.list": {
      try {
        const { handleGitWorktreeList: handleGitWorktreeList2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        return await handleGitWorktreeList2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.list unavailable: ${msg}`);
      }
    }
    // ── v5.0: git.worktree.add ───────────────────────────────────────────────
    case "git.worktree.add": {
      try {
        const { handleGitWorktreeAdd: handleGitWorktreeAdd2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        return await handleGitWorktreeAdd2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add unavailable: ${msg}`);
      }
    }
    // ── v5.0: git.worktree.remove ────────────────────────────────────────────
    case "git.worktree.remove": {
      try {
        const { handleGitWorktreeRemove: handleGitWorktreeRemove2 } = await Promise.resolve().then(() => (init_agent_git_handler(), agent_git_handler_exports));
        return await handleGitWorktreeRemove2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.remove unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.stat ────────────────────────────────────────────────────────
    case "fs.stat": {
      try {
        const { handleFsStat: handleFsStat2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsStat2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.stat unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.glob ────────────────────────────────────────────────────────
    case "fs.glob": {
      try {
        const { handleFsGlob: handleFsGlob2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsGlob2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.glob unavailable: ${msg}`);
      }
    }
    // ── v5.0: fs.writeFile ───────────────────────────────────────────────────
    case "fs.writeFile": {
      try {
        const { handleFsWriteFile: handleFsWriteFile2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsWriteFile2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.writeFile unavailable: ${msg}`);
      }
    }
    // ── v5.0: github.pr.create ───────────────────────────────────────────────
    case "github.pr.create": {
      try {
        const { handleGitHubPrCreate: handleGitHubPrCreate2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitHubPrCreate2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.create unavailable: ${msg}`);
      }
    }
    // ── v5.0: github.pr.merge ────────────────────────────────────────────────
    case "github.pr.merge": {
      try {
        const { handleGitHubPrMerge: handleGitHubPrMerge2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitHubPrMerge2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.merge unavailable: ${msg}`);
      }
    }
    // ── v5.0: github.issue.list ──────────────────────────────────────────────
    case "github.issue.list": {
      try {
        const { handleGitHubIssueList: handleGitHubIssueList2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitHubIssueList2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.list unavailable: ${msg}`);
      }
    }
    // ── v5.0: github.issue.create ────────────────────────────────────────────
    case "github.issue.create": {
      try {
        const { handleGitHubIssueCreate: handleGitHubIssueCreate2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitHubIssueCreate2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.create unavailable: ${msg}`);
      }
    }
    // ── v5.0: gitlab.mr.create ───────────────────────────────────────────────
    case "gitlab.mr.create": {
      try {
        const { handleGitLabMrCreate: handleGitLabMrCreate2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitLabMrCreate2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.create unavailable: ${msg}`);
      }
    }
    // ── v5.0: gitlab.pipeline.status ─────────────────────────────────────────
    case "gitlab.pipeline.status": {
      try {
        const { handleGitLabPipelineStatus: handleGitLabPipelineStatus2 } = await Promise.resolve().then(() => (init_external_api_connector(), external_api_connector_exports));
        return await handleGitLabPipelineStatus2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.pipeline.status unavailable: ${msg}`);
      }
    }
    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case "agent.spawn": {
      try {
        const { handleAgentSpawn: handleAgentSpawn2 } = await Promise.resolve().then(() => (init_agent_spawner(), agent_spawner_exports));
        void handleAgentSpawn2(rpc.id, rpc.params ?? {}, config, log, ws, state);
        return { jsonrpc: "2.0", id: rpc.id, result: { type: "spawn.accepted" } };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`);
      }
    }
    // ── v5.0: agent.kill ─────────────────────────────────────────────────────
    case "agent.kill": {
      try {
        const { handleAgentKill: handleAgentKill2 } = await Promise.resolve().then(() => (init_agent_spawner(), agent_spawner_exports));
        return await handleAgentKill2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`);
      }
    }
    // ── v5.0: agent.sendInput ────────────────────────────────────────────────
    // ORCH-001: Send data to a running agent PTY's stdin.
    // Used for graceful stop (Ctrl+C = '\x03') and interactive input.
    case "agent.sendInput": {
      try {
        const { handleAgentSendInput: handleAgentSendInput2 } = await Promise.resolve().then(() => (init_agent_spawner(), agent_spawner_exports));
        return await handleAgentSendInput2(rpc.id, rpc.params ?? {}, config, log);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.sendInput unavailable: ${msg}`);
      }
    }
    // ── v5.0: agent.exec ─────────────────────────────────────────────────────
    // TG-001: Non-interactive subprocess execution (for task graph steps).
    // Returns captured stdout/stderr/exitCode instead of streaming.
    // Distinct from agent.spawn (interactive PTY) — no terminal allocation.
    case "agent.exec": {
      const p = rpc.params ?? {};
      const binary = typeof p.binary === "string" ? p.binary : "";
      const span = Tracers.agentOrchSpawn.start(
        { binary, taskId: typeof p.taskId === "string" ? p.taskId : void 0 },
        extractResume2(p)
      );
      try {
        const { spawn: spawn8 } = await import("node:child_process");
        const args = Array.isArray(p.args) ? p.args.map(String) : [];
        const cwd = typeof p.cwd === "string" ? p.cwd : config.workDir;
        const stdin = typeof p.stdin === "string" ? p.stdin : null;
        const extraEnv = p.env && typeof p.env === "object" && !Array.isArray(p.env) ? p.env : {};
        const timeoutMs = typeof p.timeoutMs === "number" ? Math.min(Math.max(p.timeoutMs, 1e3), 5 * 6e4) : 3e5;
        if (!binary) {
          span.fail("binary is required");
          return makeError(rpc.id, AgentErrorCode.InvalidParams, "agent.exec: binary is required");
        }
        span.step("subprocess-spawn", { binary, cwd });
        const result = await new Promise((resolve8) => {
          let stdout = "", stderr = "", timedOut = false, settled = false;
          const spawnEnv = { ...process.env, ...extraEnv };
          const child = spawn8(binary, args, { cwd, env: spawnEnv, stdio: ["pipe", "pipe", "pipe"] });
          const finish = (r) => {
            if (settled) {return;}
            settled = true;
            clearTimeout(timer);
            resolve8(r);
          };
          const timer = setTimeout(() => {
            timedOut = true;
            try {
              child.kill("SIGKILL");
            } catch {
            }
            finish({ stdout, stderr, exitCode: null, timedOut });
          }, timeoutMs);
          child.stdout?.on("data", (d) => {
            stdout += d.toString("utf8");
          });
          child.stderr?.on("data", (d) => {
            stderr += d.toString("utf8");
          });
          child.on("error", (err) => {
            finish({ stdout, stderr: err.message, exitCode: null, timedOut });
          });
          child.on("close", (code) => {
            finish({ stdout, stderr, exitCode: code, timedOut });
          });
          if (stdin !== null) {child.stdin?.end(stdin);}
          else {child.stdin?.end();}
        });
        log.info(`agent.exec: binary=${binary} exitCode=${result.exitCode} timedOut=${result.timedOut}`);
        if (result.timedOut) {
          span.fail(`timeout after ${timeoutMs}ms`, { binary });
        } else if (result.exitCode !== 0) {
          span.fail(`exit code ${result.exitCode}`, { binary, exitCode: result.exitCode ?? -1 });
        } else {
          span.ok({ binary, exitCode: result.exitCode ?? 0 });
        }
        return { jsonrpc: "2.0", id: rpc.id, result };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        span.fail(err, { binary });
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`);
      }
    }
    // ── v5.0: ai.complete ─────────────────────────────────────────────────────
    // TG-002: Non-interactive AI completion for task planning (TaskAIPlanner.decompose)
    // and git commit message generation.
    // Called by: relay.call('ai.complete', { prompt, format: 'json'|'text', model? })
    case "ai.complete": {
      try {
        const p = rpc.params ?? {};
        const prompt = typeof p["prompt"] === "string" ? p["prompt"] : "";
        if (!prompt.trim()) {
          return makeError(rpc.id, AgentErrorCode.InvalidParams, "ai.complete: prompt is required");
        }
        const { handleAIComplete: handleAIComplete2 } = await Promise.resolve().then(() => (init_ai_complete_handler(), ai_complete_handler_exports));
        const result = await handleAIComplete2(
          {
            prompt,
            format: typeof p["format"] === "string" ? p["format"] : "text",
            taskId: typeof p["taskId"] === "string" ? p["taskId"] : void 0,
            model: typeof p["model"] === "string" ? p["model"] : void 0
          },
          config,
          log
        );
        return { jsonrpc: "2.0", id: rpc.id, result };
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`);
      }
    }
    // Runs a short shell command and returns stdout/stderr.
    // Used by devServer.browseDir on the Orca server to resolve '~' on the remote.
    // SECURITY: only used internally via relay — not exposed to browser directly.
    case "shell.eval": {
      try {
        const { handleShellEval: handleShellEval2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleShellEval2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `shell.eval unavailable: ${msg}`);
      }
    }
    // ── fs.mkdir ─────────────────────────────────────────────────────────────
    // Creates a directory (recursive) on the agent's filesystem.
    case "fs.mkdir": {
      try {
        const { handleFsMkdir: handleFsMkdir2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsMkdir2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.mkdir unavailable: ${msg}`);
      }
    }
    // ── fs.rmdir ─────────────────────────────────────────────────────────────
    // Removes an empty directory on the agent's filesystem.
    case "fs.rmdir": {
      try {
        const { handleFsRmdir: handleFsRmdir2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsRmdir2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.rmdir unavailable: ${msg}`);
      }
    }
    // ── fs.watch ─────────────────────────────────────────────────────────────
    // Starts pushing `fs.changed` notifications for a path. Idempotent/refcounted.
    case "fs.watch": {
      try {
        const { handleFsWatch: handleFsWatch2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsWatch2(rpc.id, rpc.params ?? {}, config, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.watch unavailable: ${msg}`);
      }
    }
    // ── fs.unwatch ───────────────────────────────────────────────────────────
    case "fs.unwatch": {
      try {
        const { handleFsUnwatch: handleFsUnwatch2 } = await Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports));
        return await handleFsUnwatch2(rpc.id, rpc.params ?? {}, config);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.unwatch unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.create ─────────────────────────────────────────────────────
    // TM-001/TM-006: Create a PTY session in agent mode.
    // Params: { cwd, cols?, rows?, env?, shellOverride? }
    // Returns: { id, cols, rows, cwd, shell }
    // Why all six pty.* cases below pass makeNotifier(ws, state) (not just
    // create/attach): PTYs now live in the detached pty-daemon process
    // (pty-daemon-client.ts), which can push pty.data/pty.exit/pty.replay for
    // ANY live PTY at any time, independent of which request last arrived —
    // every dispatch call rebinds the client's "current notify" to the live
    // WebSocket connection so a push always reaches it.
    case "pty.create": {
      try {
        const { handlePtyCreate: handlePtyCreate3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyCreate3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.create unavailable: ${msg}`);
      }
    }
    // ── pty.attach ───────────────────────────────────────────────────────────
    // Reattach to a PTY that survived a WebSocket disconnect (grace period)
    // or an agent process restart (the pty-daemon process survives it).
    case "pty.attach": {
      try {
        const { handlePtyAttach: handlePtyAttach3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyAttach3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.attach unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.write ──────────────────────────────────────────────────────
    // Send input data to PTY stdin.
    // Params: { id, data }
    case "pty.write": {
      try {
        const { handlePtyWrite: handlePtyWrite3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyWrite3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.write unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.resize ─────────────────────────────────────────────────────
    // Resize PTY terminal window.
    // Params: { id, cols, rows }
    case "pty.resize": {
      try {
        const { handlePtyResize: handlePtyResize3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyResize3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.resize unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.destroy ────────────────────────────────────────────────────
    // Close and cleanup a PTY session.
    // Params: { id, graceful? }
    case "pty.destroy": {
      try {
        const { handlePtyDestroy: handlePtyDestroy3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyDestroy3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.destroy unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.scrollback ─────────────────────────────────────────────────
    // Get scrollback buffer content.
    // Params: { id, lines? }
    case "pty.scrollback": {
      try {
        const { handlePtyScrollback: handlePtyScrollback3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtyScrollback3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.scrollback unavailable: ${msg}`);
      }
    }
    // ── v5.0: pty.sendSignal ─────────────────────────────────────────────────
    // Send a signal to the PTY process (SIGTERM, SIGKILL, SIGINT, etc.).
    // Params: { id, signal }
    case "pty.sendSignal": {
      try {
        const { handlePtySendSignal: handlePtySendSignal3 } = await Promise.resolve().then(() => (init_pty_daemon_client(), pty_daemon_client_exports));
        return await handlePtySendSignal3(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state));
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.sendSignal unavailable: ${msg}`);
      }
    }
    // ── Unknown method ───────────────────────────────────────────────────────
    default:
      return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`);
  }
}
function formatMcpResult(id, result) {
  const text = [
    result.stdout || "",
    result.stderr ? `[stderr]
${result.stderr}` : ""
  ].filter(Boolean).join("\n").trim();
  return {
    jsonrpc: "2.0",
    id,
    result: {
      content: [{ type: "text", text: text || "(no output)" }],
      isError: result.exitCode !== 0,
      exitCode: result.exitCode,
      meta: result.meta
    }
  };
}
function makeError(id, code, message, data) {
  return {
    jsonrpc: "2.0",
    id,
    error: { code, message, ...data !== void 0 && { data } }
  };
}

// src/relay/agent-session.ts
init_agent_wire_protocol();
init_relay_protocol();
init_trace();
init_agent_spawner();
init_pty_daemon_client();
init_fs_agent_extensions();
var sessionTracer = createTracer("agent:session");
function createSession(config, tools, log, _prebuiltCapabilities, tokenOverride) {
  let keepaliveTimer = null;
  let handshakeDone = false;
  const handshakeOkCallbacks = [];
  const dispatcher = createRpcDispatcher(tools, config, log);
  async function checkGitAvailable() {
    const { access: fsAccess, constants: constants2 } = await import("node:fs/promises");
    const { join: join11 } = await import("node:path");
    const dirs = (config.toolPath ?? process.env["PATH"] ?? "").split(":").filter(Boolean);
    for (const dir of dirs) {
      try {
        await fsAccess(join11(dir, "git"), constants2.X_OK);
        return true;
      } catch {
      }
    }
    const { execFile: execFile4 } = await import("node:child_process");
    return new Promise((resolve8) => {
      const child = execFile4("git", ["--version"], { timeout: 3e3 });
      child.on("close", (code) => resolve8(code === 0));
      child.on("error", () => resolve8(false));
    });
  }
  async function checkPtyAvailable() {
    try {
      await import("node-pty");
      return true;
    } catch {
      return false;
    }
  }
  async function buildCapabilities() {
    const caps = [
      "fs",
      "fs.watch",
      "preflight",
      "ai.providers",
      "agent.spawn",
      "agent.exec",
      "agent.sendInput",
      "agent.kill"
    ];
    const [hasGit, hasPty] = await Promise.all([checkGitAvailable(), checkPtyAvailable()]);
    log.info(`capability check: git=${hasGit} pty=${hasPty}`);
    if (hasGit) {
      caps.push("git", "git.exec", "git.execStream");
      caps.push("worktrees", "git.worktree.list", "git.worktree.add", "git.worktree.remove");
    }
    if (hasPty) {
      caps.push(
        "pty",
        "pty.create",
        "pty.write",
        "pty.resize",
        "pty.destroy",
        "pty.scrollback",
        "pty.stream",
        "pty.attach"
      );
    }
    log.info(`capabilities: [${caps.join(", ")}]`);
    return caps;
  }
  const STATIC_CAPABILITIES_FALLBACK = [
    "fs",
    "fs.watch",
    "git",
    "preflight",
    "ai.providers",
    "agent.spawn",
    "worktrees",
    "git.exec",
    "git.execStream",
    "git.worktree.list",
    "git.worktree.add",
    "git.worktree.remove",
    "pty",
    "pty.create",
    "pty.write",
    "pty.resize",
    "pty.destroy",
    "pty.scrollback",
    "pty.stream",
    "pty.attach"
  ];
  async function sendHandshake(ws, wireState) {
    let capabilities;
    if (_prebuiltCapabilities) {
      capabilities = _prebuiltCapabilities;
    } else {
      try {
        capabilities = await Promise.race([
          buildCapabilities(),
          new Promise(
            (_res, reject) => setTimeout(() => reject(new Error("capability check timeout")), 5e3)
          )
        ]);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        log.warn(`buildCapabilities failed (${msg}) \u2014 using static fallback`);
        capabilities = STATIC_CAPABILITIES_FALLBACK;
      }
    }
    const rpc = {
      jsonrpc: "2.0",
      id: 1,
      method: AGENT_HANDSHAKE_METHOD,
      params: {
        agentVersion: "5.0.0",
        platform: process.platform,
        arch: process.arch,
        nodeVersion: process.version,
        capabilities,
        // agentToken is only sent in direct-websocket mode; empty string = omit.
        // tokenOverride takes precedence so renewed tokens are used transparently.
        ...tokenOverride || config.agentToken ? { agentToken: tokenOverride ?? config.agentToken } : {},
        devServerId: config.devServerId,
        tools: tools.map((t) => t.name)
      }
    };
    ws.send(encodeDataFrame(wireState, JSON.stringify(rpc)));
    log.info(
      `Handshake sent: devServerId=${config.devServerId} tools=[${tools.map((t) => t.name).join(",")}]`
    );
  }
  function startKeepalive(ws, wireState) {
    keepaliveTimer = setInterval(() => {
      if (ws.readyState === 1) {
        ws.send(encodeKeepaliveFrame(wireState));
      }
    }, AGENT_KEEPALIVE_INTERVAL_MS);
  }
  return {
    start(ws) {
      const wireState = createWireState();
      const span = sessionTracer.start({ devServerId: config.devServerId });
      const doHandshake = () => {
        void sendHandshake(ws, wireState).then(() => {
          span.step("handshake-sent");
          startKeepalive(ws, wireState);
        }).catch((err) => {
          const msg = err instanceof Error ? err.message : String(err);
          log.error(`sendHandshake failed: ${msg}`);
          span.fail(err, { phase: "handshake" });
          ws.close(1011, "Handshake error");
        });
      };
      if (ws.readyState === 1) {
        doHandshake();
      } else {
        ws.once("open", () => {
          log.info("WebSocket opened");
          doHandshake();
        });
      }
      ws.on("message", (data) => {
        if (!Buffer.isBuffer(data)) {
          return;
        }
        const frame = decodeFrame(wireState, data);
        if (!frame) {
          log.warn("Received malformed frame (too short) \u2014 ignoring");
          return;
        }
        if (frame.type === MessageType.KeepAlive) {
          if (ws.readyState === 1) {
            ws.send(encodeKeepaliveFrame(wireState));
          }
          return;
        }
        if (frame.payload.length === 0) {
          return;
        }
        const rpc = parseJsonPayload(frame.payload);
        if (!rpc) {
          log.warn("Received non-JSON frame payload \u2014 ignoring");
          return;
        }
        if (!handshakeDone) {
          if (rpc.result?.ok === true) {
            handshakeDone = true;
            const sessionId = rpc.result.sessionId ?? "unknown";
            const orcaVersion = rpc.result.orcaVersion ?? "unknown";
            log.info(`Handshake OK: sessionId=${sessionId} orcaVersion=${orcaVersion}`);
            span.step("handshake-ok", { sessionId, orcaVersion });
            handshakeOkCallbacks.forEach((cb) => cb());
          } else if (rpc.error) {
            log.error(`Handshake failed: code=${rpc.error.code} message=${rpc.error.message}`);
            span.fail(`handshake: ${rpc.error.message}`, { code: rpc.error.code });
            ws.close(1008, "Handshake failed");
          }
          return;
        }
        if (typeof rpc.method === "string") {
          void dispatcher.dispatch(ws, wireState, rpc);
        }
      });
      ws.on("close", (code, reason) => {
        this.stop();
        const reasonStr = reason.toString();
        if (code === 1e3) {
          span.ok({ code, reason: reasonStr });
        } else {
          span.fail(`ws close code=${code}`, { code, reason: reasonStr });
        }
        log.info(`Session closed code=${code} reason=${reasonStr}`);
      });
      ws.on("error", (err) => {
        span.fail(err, { phase: "ws-error" });
        log.error(`WebSocket error: ${err.message}`);
      });
    },
    stop() {
      if (keepaliveTimer !== null) {
        clearInterval(keepaliveTimer);
        keepaliveTimer = null;
      }
      cleanupAllPtys(log);
      void notifyDaemonSessionClosed(log);
      cleanupAgentWatches();
    },
    onHandshakeOk(callback) {
      handshakeOkCallbacks.push(callback);
    }
  };
}

// src/relay/agent-connection-direct.ts
init_trace();

// src/relay/agent-token-manager.ts
init_trace();
var tokenTracer = createTracer("agent:tokenManager");
var TOKEN_RENEW_RATIO = 0.8;
var AGENT_TOKEN_DEFAULT_TTL_SEC = 86400;
var RETRY_DELAYS_MS = [5e3, 15e3, 3e4, 6e4];
var MAX_RENEWAL_ATTEMPTS = 5;
var AgentTokenManager = class {
  current = null;
  next = null;
  renewTimer = null;
  disposed = false;
  opts;
  constructor(opts) {
    this.opts = { ttlSec: AGENT_TOKEN_DEFAULT_TTL_SEC, ...opts };
  }
  // ── Public API ─────────────────────────────────────────────────────────────
  /**
   * Fetch the initial token. Must be called once before consume().
   * Exits the process if unable to reach the server after MAX_RENEWAL_ATTEMPTS.
   */
  async init() {
    this.opts.log.info(`Fetching agent token from ${this.opts.orcaHttpUrl} ...`);
    this.current = await this.fetchWithRetry("initial", MAX_RENEWAL_ATTEMPTS);
    this.opts.log.info(
      `Token OK (ttl=${this.current.ttlSec}s). Starting agent (mode=direct-websocket)...`
    );
    tokenTracer.start({ op: "init", devServerId: this.opts.devServerId, ttl: this.current.ttlSec }).ok();
    this.scheduleRenewal(this.current.ttlSec);
  }
  /**
   * Return the best available token for the next WS connection attempt.
   * If a pre-fetched renewal token is ready, it becomes current.
   * Call this just before each WebSocket connect.
   */
  consume() {
    if (this.next) {
      this.opts.log.info("[TokenManager] Using pre-fetched renewal token for reconnect");
      this.current = this.next;
      this.next = null;
      this.scheduleRenewal(this.current.ttlSec);
    }
    if (!this.current) {
      throw new Error("AgentTokenManager.consume() called before init()");
    }
    return this.current.token;
  }
  /** Release timers. Call on process exit. */
  dispose() {
    this.disposed = true;
    if (this.renewTimer) {
      clearTimeout(this.renewTimer);
      this.renewTimer = null;
    }
  }
  /**
   * Force a synchronous renewal right now (bypassing the timer).
   * Useful when the server rejects the current token (e.g. after server restart).
   */
  async forceRenew() {
    this.opts.log.info("[TokenManager] Force token renewal triggered");
    if (this.renewTimer) {
      clearTimeout(this.renewTimer);
      this.renewTimer = null;
    }
    await this.doRenewal();
  }
  // ── Internals ──────────────────────────────────────────────────────────────
  scheduleRenewal(ttlSec) {
    if (this.renewTimer) {
      clearTimeout(this.renewTimer);
      this.renewTimer = null;
    }
    if (this.disposed) {return;}
    const delayMs = Math.floor(ttlSec * TOKEN_RENEW_RATIO * 1e3);
    const renewAt = new Date(Date.now() + delayMs).toISOString();
    this.opts.log.info(
      `[TokenManager] Next token renewal in ${Math.round(delayMs / 6e4)}m (at ${renewAt})`
    );
    this.renewTimer = setTimeout(() => {
      this.renewTimer = null;
      void this.doRenewal();
    }, delayMs);
    if (typeof this.renewTimer.unref === "function") {
      this.renewTimer.unref();
    }
  }
  async doRenewal() {
    if (this.disposed) {return;}
    this.opts.log.info("[TokenManager] Proactive token renewal starting...");
    const span = tokenTracer.start({ op: "renew", devServerId: this.opts.devServerId });
    try {
      const fetched = await this.fetchWithRetry("renewal", MAX_RENEWAL_ATTEMPTS);
      this.next = fetched;
      span.ok({ ttl: fetched.ttlSec });
      this.opts.log.info(
        `[TokenManager] Renewal token ready (ttl=${fetched.ttlSec}s). Will use on next reconnect.`
      );
      this.scheduleRenewal(fetched.ttlSec);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      span.fail(err, { devServerId: this.opts.devServerId });
      this.opts.log.warn(
        `[TokenManager] Renewal failed: ${msg}. Retrying in 5m with current token.`
      );
      this.scheduleRenewal(300);
    }
  }
  async fetchWithRetry(label, maxAttempts) {
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        return await this.fetchOnce();
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        if (attempt >= maxAttempts) {
          if (label === "initial") {
            this.opts.log.error(
              `FATAL: Cannot reach Orca Server after ${maxAttempts} attempts. Exit.`
            );
            process.exit(1);
          }
          throw new Error(`Token ${label} failed after ${maxAttempts} attempts: ${msg}`);
        }
        const delayMs = RETRY_DELAYS_MS[Math.min(attempt - 1, RETRY_DELAYS_MS.length - 1)];
        this.opts.log.warn(
          `[TokenManager] Token fetch failed (attempt ${attempt}/${maxAttempts}). Retry in ${delayMs / 1e3}s... (${msg})`
        );
        await sleep(delayMs);
      }
    }
    throw new Error("fetchWithRetry: exhausted attempts");
  }
  async fetchOnce() {
    const url = `${this.opts.orcaHttpUrl}/api/agent-token`;
    const body = JSON.stringify({
      devServerId: this.opts.devServerId,
      name: this.opts.name,
      ttl: this.opts.ttlSec,
      permanent: true
    });
    const res = await httpPost(url, body, {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${this.opts.apiSecret}`
    }, 1e4);
    if (res.status === 401) {throw new Error("Unauthorized \u2014 check ORCA_AGENT_API_SECRET");}
    if (res.status !== 200) {throw new Error(`HTTP ${res.status}: ${res.body.slice(0, 200)}`);}
    let parsed;
    try {
      parsed = JSON.parse(res.body);
    } catch {
      throw new Error(`Invalid JSON response: ${res.body.slice(0, 100)}`);
    }
    const token = parsed["token"];
    if (typeof token !== "string" || !token) {
      throw new Error(`No token in response: ${res.body.slice(0, 100)}`);
    }
    const ttlSec = typeof parsed["expiresIn"] === "number" ? parsed["expiresIn"] : this.opts.ttlSec;
    return { token, ttlSec, fetchedAt: Date.now() };
  }
};
function sleep(ms) {
  return new Promise((resolve8) => setTimeout(resolve8, ms));
}
async function httpPost(url, body, headers, timeoutMs) {
  const { request: httpRequest } = await import("node:http");
  const { request: httpsRequest } = await import("node:https");
  const { URL: NodeURL } = await import("node:url");
  return new Promise((resolve8, reject) => {
    const parsed = new NodeURL(url);
    const isHttps = parsed.protocol === "https:";
    const doRequest = isHttps ? httpsRequest : httpRequest;
    const req = doRequest(
      {
        hostname: parsed.hostname,
        port: parsed.port || (isHttps ? 443 : 80),
        path: parsed.pathname + parsed.search,
        method: "POST",
        headers: { ...headers, "Content-Length": Buffer.byteLength(body) },
        rejectUnauthorized: process.env["NODE_TLS_REJECT_UNAUTHORIZED"] !== "0"
      },
      (res) => {
        let data = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          data += chunk;
        });
        res.on("end", () => resolve8({ status: res.statusCode ?? 0, body: data }));
      }
    );
    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error(`HTTP request timed out after ${timeoutMs}ms`));
    });
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

// src/relay/agent-connection-direct.ts
var connTracer = createTracer("agent:connection");
var RECONNECT_DELAYS_MS = [1e3, 2e3, 5e3, 15e3, 3e4];
async function connectDirect(config, tools, log) {
  let tokenManager = null;
  if (config.apiSecret) {
    tokenManager = new AgentTokenManager({
      orcaHttpUrl: config.orcaHttpUrl,
      devServerId: config.devServerId,
      name: config.devServerId,
      apiSecret: config.apiSecret,
      ttlSec: AGENT_TOKEN_DEFAULT_TTL_SEC,
      log
    });
    await tokenManager.init();
    process.on("SIGINT", () => {
      tokenManager?.dispose();
      process.exit(0);
    });
    process.on("SIGTERM", () => {
      tokenManager?.dispose();
      process.exit(0);
    });
  } else {
    if (!config.agentToken) {
      log.error("Neither ORCA_AGENT_API_SECRET nor AGENT_TOKEN is set.");
      log.error("Set ORCA_AGENT_API_SECRET to enable automatic token renewal.");
      process.exit(1);
    }
    log.warn("[TokenManager] ORCA_AGENT_API_SECRET not set \u2014 using static AGENT_TOKEN (no renewal).");
  }
  let reconnectAttempt = 0;
  const runConnection = () => new Promise((resolve8) => {
    const token = tokenManager ? tokenManager.consume() : config.agentToken;
    const connSpan = connTracer.start({ url: config.orcaUrl, attempt: reconnectAttempt });
    let lastHandshakeOk = false;
    log.info(`Connecting to ${config.orcaUrl} ...`);
    const ws = new wrapper_default(config.orcaUrl, {
      headers: { "User-Agent": "orca-dev-agent/2.1.0" },
      rejectUnauthorized: config.tlsRejectUnauthorized
    });
    const session = createSession(config, tools, log, void 0, token);
    session.onHandshakeOk(() => {
      lastHandshakeOk = true;
      reconnectAttempt = 0;
      connSpan.step("handshake-ok");
      log.info("Connection established and authenticated.");
    });
    session.start(ws);
    ws.once("close", (code) => {
      session.stop();
      if (code === 1e3) {
        connSpan.ok({ code });
        log.info("Connection closed cleanly (code=1000). Shutting down.");
        resolve8("exit");
        return;
      }
      if (lastHandshakeOk) {
        connSpan.fail(`connection dropped after handshake`, { code });
        log.warn(`Connection dropped (code=${code}). Reconnecting...`);
        resolve8("reconnect-renew");
        return;
      } else {
        connSpan.fail(`closed before handshake`, { code });
        log.warn(`Connection closed before handshake (code=${code}). Reconnecting...`);
        resolve8("reconnect-auth-failed");
        return;
      }
    });
    ws.once("error", (err) => {
      connSpan.fail(err);
      log.warn(`WebSocket error: ${err.message}. Reconnecting...`);
    });
  });
  while (true) {
    const result = await runConnection();
    if (result === "exit") {
      tokenManager?.dispose();
      setTimeout(() => process.exit(0), 100);
      return new Promise(() => {
      });
    }
    if (tokenManager) {
      log.warn(
        result === "reconnect-auth-failed" ? "Handshake rejected (likely unregistered token). Forcing proactive token renewal..." : "Connection dropped after a successful handshake \u2014 its token is now stale server-side. Forcing proactive token renewal..."
      );
      try {
        await tokenManager.forceRenew();
      } catch (err) {
        log.error(`Failed to force renew token: ${err instanceof Error ? err.message : String(err)}`);
      }
    }
    const delay = RECONNECT_DELAYS_MS[Math.min(reconnectAttempt, RECONNECT_DELAYS_MS.length - 1)];
    reconnectAttempt += 1;
    log.info(`Reconnect in ${delay / 1e3}s (attempt ${reconnectAttempt})...`);
    await sleep2(delay);
  }
}
function sleep2(ms) {
  return new Promise((resolve8) => setTimeout(resolve8, ms));
}

// src/relay/agent-connection-relay.ts
init_trace();
var relayConnTracer = createTracer("agent:connectionRelay");
async function listenRelay(config, tools, log) {
  const token = config.agentToken?.trim();
  if (!token) {
    log.error("FATAL: agentToken (ORCA_AGENT_TOKEN) is not set or empty.");
    log.error("In relay-websocket mode, a shared secret is required for authentication.");
    log.error("On the Dev Server, run:");
    log.error("  export ORCA_AGENT_TOKEN=$(openssl rand -hex 32)");
    log.error("  node ~/orca-agent/agent.js");
    process.exit(1);
  }
  return new Promise((_, reject) => {
    const wss = new import_websocket_server.default({ port: config.agentPort, path: "/orca-relay" });
    wss.once("listening", () => {
      log.info(`\u2705 Relay server ready: ws://0.0.0.0:${config.agentPort}/orca-relay`);
      log.info(`Orca UI config \u2192 Type: relay-websocket  URL: ws://<devServerHost>:${config.agentPort}/orca-relay`);
      log.info(`Set the token in Orca UI \u2192 Dev Server settings (matches ORCA_AGENT_TOKEN on this machine)`);
    });
    wss.on("connection", (ws, req) => {
      const remoteAddr = req.socket.remoteAddress ?? "unknown";
      const span = relayConnTracer.start({ remoteAddr });
      if (!authenticate(ws, req, token, log, span)) {return;}
      log.info(`Orca Server connected from ${remoteAddr}`);
      span.step("accepted", { remoteAddr });
      const session = createSession(config, tools, log);
      session.start(ws);
      ws.once("close", (code) => {
        session.stop();
        if (code === 1e3) {
          span.ok({ code, remoteAddr });
        } else {
          span.fail(`ws close code=${code}`, { code, remoteAddr });
        }
        log.info(`Orca Server disconnected from ${remoteAddr} (code=${code})`);
      });
    });
    wss.once("error", (err) => {
      log.error(`Relay server fatal error: ${err.message}`);
      reject(err);
    });
  });
}
function authenticate(ws, req, expectedToken, log, span) {
  const rawUrl = req.url ?? "";
  let queryToken = "";
  try {
    queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get("token") ?? "";
  } catch {
  }
  const authHeader = req.headers["authorization"] ?? "";
  const bearerToken = authHeader.replace(/^Bearer\s+/i, "").trim();
  const incoming = queryToken || bearerToken;
  const source = queryToken ? "query" : bearerToken ? "header" : "none";
  if (incoming !== expectedToken) {
    span?.fail("unauthorized", { source });
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? "unknown"}`);
    ws.close(1008, "Unauthorized");
    return false;
  }
  span?.step("tokenAccepted", { source });
  return true;
}

// src/relay/agent-entry.ts
async function main() {
  const daemonSocketPath = process.env["ORCA_PTY_DAEMON_SOCKET"];
  if (daemonSocketPath) {
    const daemonLog = createAgentLogger(process.env["ORCA_LOG_LEVEL"] ?? "info");
    const { runPtyDaemon: runPtyDaemon2 } = await Promise.resolve().then(() => (init_pty_daemon_server(), pty_daemon_server_exports));
    await runPtyDaemon2(daemonSocketPath, daemonLog);
    return;
  }
  const config = loadAgentConfig();
  const log = createAgentLogger(config.logLevel);
  log.info("Orca Dev Agent v2.1.0");
  log.info(`Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`);
  log.info("Discovering tools...");
  const tools = await discoverTools(config);
  log.info(`Tools ready: ${tools.length} (${tools.map((t) => t.name).join(", ")})`);
  const shutdown = (signal) => {
    log.info(`Shutting down (${signal})`);
    Promise.resolve().then(() => (init_fs_agent_extensions(), fs_agent_extensions_exports)).then((m) => m.cleanupAgentWatches()).catch(() => {
    }).finally(() => process.exit(0));
  };
  process.on("SIGINT", () => shutdown("SIGINT"));
  process.on("SIGTERM", () => shutdown("SIGTERM"));
  if (config.mode === "relay-websocket") {
    await listenRelay(config, tools, log);
  } else {
    await connectDirect(config, tools, log);
  }
}
main().catch((err) => {
  const msg = err instanceof Error ? err.message : String(err);
  console.error(`[agent:FATAL] ${msg}`);
  process.exit(1);
});
