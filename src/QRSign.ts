import WebSocket from 'ws';
import { toString as toQR } from 'qrcode';
import { IBasicSignInfo } from './requests';
import { qr } from './consts';
import { copyToClipBoard, makeDebugLogger } from './utils';
import { WechatDevtools } from './cdp';
import { IContext } from './sign';

const debugLogger = makeDebugLogger('QRSign::');

interface IChannelMessage {
  id: string;
  channel: string;
  successful?: boolean;
}
interface IServerMessage extends IChannelMessage {
  version: string;
  successful: boolean;
  advice: {
    reconnect: string;
    interval: number;
    timeout: number;
  };
  supportedConnectionTypes?: string[];
}

interface IHandShakeMessage extends IServerMessage {
  clientId: string;
}

enum QRType {
  default,
  code,
  unknown,
  result,
}

interface IQRStudentResult {
  id?: number;
  name?: string;
  studentNumber?: string;
  rank: number;
  // teamId: 0;
  // isNew: false;
  // distance: 0;
  // isOutOfBound: -1;
}

interface IQRMessage {
  channel: string;
  data: {
    type: number; // 1 for code; 3 for student
    qrUrl?: string; // for type 1
    student?: IQRStudentResult; // for type 3
  };
  id: string;
}

type successCallback = (result: IQRStudentResult) => void;
type errorCallback = (err: any) => void;

interface IQRSignOptions extends Partial<IBasicSignInfo> {
  setOpenId?: (openId: string) => void;
  devtools?: WechatDevtools;
}

export class QRSign {
  // static
  static endpoint = 'wss://www.teachermate.com.cn/faye';
  // fields
  private _seqId = 0;
  private courseId: number | undefined;
  private signId: number | undefined;
  private clientId = '';
  private client: WebSocket | null = null;
  private interval: NodeJS.Timeout | undefined;
  private onError: errorCallback | null = null;
  private onSuccess: successCallback | null = null;
  private ctx: IContext;
  private currentQRUrl = '';
  private connected = false;
  private subscribedKey: string | null = null;

  static testQRSubscription = (msg: IChannelMessage): msg is IQRMessage =>
    /attendance\/\d+\/\d+\/qr/.test(msg.channel);

  constructor(ctx: IContext, info: IQRSignOptions = {}) {
    this.courseId = info.courseId;
    this.signId = info.signId;
    this.ctx = ctx;
  }

  startSync(cb?: successCallback, err?: (err: any) => void) {
    this.onError = err ?? null;
    this.onSuccess = cb ?? null;

    if (!this.client) {
      this.client = new WebSocket(QRSign.endpoint);
      this.client.on('open', () => {
        this.handshake();
      });
      this.client.on('message', (data) => {
        try {
          // RAW frame logging for diagnosis
          console.log(`[RAW][${new Date().toISOString()}]`, data.toString());
        } catch {}
        debugLogger(`receiveMessage`, data);
        this.handleMessage(data.toString());
      });
      this.onError && this.client.on('error', this.onError);
    }
  }

  start = () =>
    new Promise<IQRStudentResult>((resolve, reject) => {
      // 若尚未建立连接，则建立；若已建立，仅更新回调
      this.startSync(resolve, reject);
      // 如果已经连接且已有 signId/courseId，则确保已订阅
      if (this.connected && this.courseId && this.signId) {
        this.subscribe();
      }
    });

  destory() {
    if (this.interval) {
      clearInterval(this.interval);
    }
    this.client?.close();
  }

  // getters
  private get seqId() {
    return `${this._seqId++}`;
  }

  private sendMessage = (msg?: object) => {
    debugLogger(`sendMessage`, msg);
    const raw = JSON.stringify(msg ? [msg] : []);
    this.client?.send(raw);
  };

  private handleQRSubscription = async (message: IQRMessage) => {
    const { data } = message;
    switch (data.type) {
      case QRType.code: {
        const { qrUrl } = data;
        if (!qrUrl || qrUrl === this.currentQRUrl) {
          return;
        }
        this.currentQRUrl = qrUrl;
        // TODO: should devtools conflict with printer?
        if (this.ctx.devtools) {
          // automation via devtools
          const result = await this.ctx.devtools.finishQRSign(qrUrl);
          // reset openId is mandatory, for scanning QR code triggering another oauth
          this.ctx.openId = result.openId;
          // race with QRType.result
          if (result.success) {
            this.onSuccess?.(result as IQRStudentResult);
          }
        }
        // manually print or execute command
        switch (qr.mode) {
          case 'terminal': {
            toQR(this.currentQRUrl, { type: 'terminal' }).then((qrStr) => {
              console.log(qrStr);
              console.log(`[TS][QR-Printed] @ ${new Date().toISOString()}`);
            });
            break;
          }
          case 'copy': {
            copyToClipBoard(this.currentQRUrl);
          }
          case 'plain': {
            console.log(this.currentQRUrl);
            break;
          }
          default:
            break;
        }

        break;
      }
      case QRType.result: {
        const { student } = data;
        // TODO: get student info from devtools
        if (student && student.name === this.ctx.studentName) {
          this.onSuccess?.(student);
        }
        break;
      }
      default:
        break;
    }
  };

  private handleMessage = (data: string) => {
    try {
      const messages = JSON.parse(data);
      if (!Array.isArray(messages)) return;
      // heartbeat response: empty array
      if (messages.length === 0) return;

      for (const raw of messages) {
        const message = raw as IChannelMessage;
        const { channel, successful } = message;
        if (!successful) {
          if (QRSign.testQRSubscription(message)) {
            debugLogger(`${channel}: payload`);
            this.handleQRSubscription(message as IQRMessage);
          } else {
            debugLogger(`${channel}: non-success & ignored`);
          }
        } else {
          debugLogger(`${channel}: successful!`);
          switch (channel) {
            case '/meta/handshake': {
              const { clientId } = message as IHandShakeMessage;
              this.clientId = clientId;
              this.connect();
              break;
            }
            case '/meta/connect': {
              const {
                advice: { timeout },
              } = message as IServerMessage;
              this.startHeartbeat(timeout);
              this.connected = true;
              // 仅当已具备 sign 信息时再订阅
              if (this.courseId && this.signId) {
                this.subscribe();
              }
              break;
            }
            case '/meta/subscribe': {
              // subscription ack, no action
              break;
            }
            default: {
              break;
            }
          }
        }
      }
    } catch (err) {
      console.error(`QR: ${err}`);
    }
  };

  private handshake = () =>
    this.sendMessage({
      channel: '/meta/handshake',
      version: '1.0',
      supportedConnectionTypes: [
        'websocket',
        'eventsource',
        'long-polling',
        'cross-origin-long-polling',
        'callback-polling',
      ],
      id: this.seqId,
    });

  private connect = () => {
    this.sendMessage({
      channel: '/meta/connect',
      clientId: this.clientId,
      connectionType: 'websocket',
      id: this.seqId,
    });
  };

  private startHeartbeat = (timeout: number) => {
    this.sendMessage();
    this.interval = setInterval(() => {
      this.sendMessage();
      this.connect();
    }, timeout / 2);
  };

  private subscribe = () => {
    if (!this.courseId || !this.signId) return;
    const key = `${this.courseId}/${this.signId}`;
    if (this.subscribedKey === key) return; // de-duplicate
    this.subscribedKey = key;
    this.sendMessage({
      channel: '/meta/subscribe',
      clientId: this.clientId,
      subscription: `/attendance/${this.courseId}/${this.signId}/qr`,
      id: this.seqId,
    });
  };

  // 供延迟绑定 sign 信息后快速订阅
  public attach = (info: IBasicSignInfo) => {
    this.courseId = info.courseId;
    this.signId = info.signId;
    this.currentQRUrl = '';
    if (this.connected) {
      this.subscribe();
    }
  };

  // 仅更新回调，不重建连接
  public setHandlers = (cb?: successCallback, err?: errorCallback) => {
    this.onSuccess = cb ?? null;
    this.onError = err ?? null;
  };
}
