export class MarmotError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "MarmotError";
  }
}

export class AuthError extends MarmotError {
  constructor(message: string) {
    super(message);
    this.name = "AuthError";
  }
}

export class NotFoundError extends MarmotError {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}

export class ServerError extends MarmotError {
  statusCode: number | undefined;

  constructor(message: string, statusCode?: number) {
    super(message);
    this.name = "ServerError";
    this.statusCode = statusCode;
  }
}
