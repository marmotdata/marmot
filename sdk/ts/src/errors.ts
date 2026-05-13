export class MarmotError extends Error {
  statusCode: number | undefined;

  constructor(message: string, statusCode?: number) {
    super(message);
    this.name = "MarmotError";
    this.statusCode = statusCode;
  }
}

export class AuthError extends MarmotError {
  constructor(message: string, statusCode?: number) {
    super(message, statusCode);
    this.name = "AuthError";
  }
}

export class NotFoundError extends MarmotError {
  constructor(message: string, statusCode?: number) {
    super(message, statusCode);
    this.name = "NotFoundError";
  }
}

export class ValidationError extends MarmotError {
  constructor(message: string, statusCode?: number) {
    super(message, statusCode);
    this.name = "ValidationError";
  }
}

export class RateLimitError extends MarmotError {
  constructor(message: string, statusCode?: number) {
    super(message, statusCode);
    this.name = "RateLimitError";
  }
}

export class ServerError extends MarmotError {
  constructor(message: string, statusCode?: number) {
    super(message, statusCode);
    this.name = "ServerError";
  }
}

export function isNotFound(err: unknown): err is NotFoundError {
  return err instanceof NotFoundError;
}

export function isRateLimit(err: unknown): err is RateLimitError {
  return err instanceof RateLimitError;
}
