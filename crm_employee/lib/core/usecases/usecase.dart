import 'package:dartz/dartz.dart';
import 'package:equatable/equatable.dart';

import '../error/failures.dart';

/// One use case = one business action, always callable the same way
/// (`usecase(params)`) regardless of what it actually does — the shape
/// every use case in every feature follows, so a Bloc never needs to know
/// each use case's individual method name.
abstract class UseCase<Result, Params> {
  Future<Either<Failure, Result>> call(Params params);
}

/// For a use case that genuinely takes no input (e.g. "log out", "check
/// whether a session is already stored") — `NoParams()` instead of
/// `void`, so `UseCase<Type, void>` doesn't force every call site to
/// write `call(null)`.
class NoParams extends Equatable {
  const NoParams();

  @override
  List<Object?> get props => [];
}
