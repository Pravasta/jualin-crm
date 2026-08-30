import '../repositories/auth_repository.dart';

/// Deliberately NOT `UseCase<bool, NoParams>` (the `Either<Failure, T>`
/// base other use cases in this feature use) — reading whether a token
/// is in secure storage has no meaningful failure mode to represent as a
/// `Failure`; wrapping a call that can only ever succeed in `Either` just
/// to look uniform would be hollow, always `Right(...)` and never
/// `Left(...)`. A plain boolean query is the honest shape here.
class CheckStoredSessionUseCase {
  final AuthRepository repository;

  const CheckStoredSessionUseCase(this.repository);

  Future<bool> call() => repository.hasStoredSession();
}
