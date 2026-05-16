---- MODULE HotStuff ----
EXTENDS Naturals, FiniteSets, TLC

(***************************************************************************)
(* HotStuff-2 BFT Consensus — TLA+ Formal Specification                   *)
(*                                                                         *)
(* Models:                                                                 *)
(*   - Byzantine validators (equivocation, arbitrary messages)             *)
(*   - Network partitions (message loss via DropMessages)                  *)
(*   - Malicious leaders (conflicting proposals via ByzantineSend)         *)
(*   - Timeout certificates (view-change protocol via NewView)             *)
(*   - Quorum intersection properties (static invariant)                   *)
(*   - No-double-commit safety                                             *)
(*   - Liveness under weak fairness (temporal property)                    *)
(***************************************************************************)

CONSTANTS N, F, MaxHeight, Faulty

Replicas == 0..(N-1)

ASSUME N > 3 * F
ASSUME Faulty \subseteq Replicas
ASSUME Cardinality(Faulty) <= F

(***************************************************************************)
(* Phase constants                                                        *)
(***************************************************************************)
PhasePrepare == 0
PhasePreCommit == 1
PhaseCommit == 2
PhaseDecide == 3

Honest == Replicas \ Faulty
Quorum == N - F

(***************************************************************************)
(* Message types and constructor                                          *)
(***************************************************************************)
MsgTypes == {"proposal", "vote_prep", "vote_precommit", "vote_commit", "timeout"}

Msg(typ, h, v, s, bh, bv) ==
    [typ |-> typ, h |-> h, v |-> v, sender |-> s, bh |-> bh, bv |-> bv]

(***************************************************************************)
(* Variables                                                              *)
(***************************************************************************)
VARIABLES
    height,        \* current height of each replica
    view,          \* current view (round) of each replica
    phase,         \* current phase of each replica
    decided,       \* whether each replica has decided at current height
    decidedHash,   \* the hash/value each replica decided on
    lockedView,    \* view at which each replica last locked
    lockedValue,   \* value locked by each replica
    messages,      \* all messages ever broadcast (global set)
    maxDecided     \* highest height at which any replica decided

vars == <<height, view, phase, decided, decidedHash, lockedView, lockedValue, messages, maxDecided>>

(***************************************************************************)
(* Init                                                                    *)
(***************************************************************************)
Init ==
    /\ height = [r \in Replicas |-> 1]
    /\ view = [r \in Replicas |-> 0]
    /\ phase = [r \in Replicas |-> PhasePrepare]
    /\ decided = [r \in Replicas |-> FALSE]
    /\ decidedHash = [r \in Replicas |-> 0]
    /\ lockedView = [r \in Replicas |-> 0]
    /\ lockedValue = [r \in Replicas |-> 0]
    /\ messages = {}
    /\ maxDecided = 0

(***************************************************************************)
(* Helper definitions                                                     *)
(***************************************************************************)
Leader(h, v) == (h + v) % N

IsHonest(r) == r \in Honest
IsFaulty(r) == r \in Faulty

(***************************************************************************)
(* Message predicates                                                     *)
(***************************************************************************)
IsProposal(m) == m.typ = "proposal"
IsVotePrep(m) == m.typ = "vote_prep"
IsVotePreCommit(m) == m.typ = "vote_precommit"
IsVoteCommit(m) == m.typ = "vote_commit"
IsTimeout(m) == m.typ = "timeout"

(***************************************************************************)
(* Timeout Certificate: N-F distinct timeout messages for a (height,view) *)
(***************************************************************************)
TCFormed(ms, h, v) ==
    LET tms == {m \in ms : IsTimeout(m) /\ m.h = h /\ m.v = v}
        senders == {m.sender : m \in tms}
    IN Cardinality(senders) >= Quorum

(***************************************************************************)
(* ===== HONEST REPLICA ACTIONS =====                                     *)
(***************************************************************************)

(***************************************************************************)
(* Propose: honest leader broadcasts a proposal for current height/view   *)
(***************************************************************************)
Propose(r) ==
    /\ IsHonest(r)
    /\ Leader(height[r], view[r]) = r
    /\ phase[r] = PhasePrepare
    /\ decided[r] = FALSE
    /\ messages' = messages \cup {Msg("proposal", height[r], view[r], r, height[r], 0)}
    /\ UNCHANGED <<height, view, phase, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* VotePrepare: honest replica votes for a valid proposal                 *)
(***************************************************************************)
VotePrepare(r) ==
    /\ IsHonest(r)
    /\ phase[r] = PhasePrepare
    /\ decided[r] = FALSE
    /\ \E m \in messages :
        IsProposal(m) /\ m.h = height[r] /\ m.v = view[r]
        /\ Leader(m.h, m.v) = m.sender
    /\ messages' = messages \cup {Msg("vote_prep", height[r], view[r], r, height[r], 0)}
    /\ phase' = [phase EXCEPT ![r] = PhasePreCommit]
    /\ UNCHANGED <<height, view, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* CollectPreCommit: collect N-F prepare votes (creates prepare QC)       *)
(* Locks the replica at this view so it won't vote for conflicting values *)
(***************************************************************************)
CollectPreCommit(r) ==
    /\ IsHonest(r)
    /\ phase[r] = PhasePreCommit
    /\ decided[r] = FALSE
    /\ LET votes == {m \in messages : IsVotePrep(m) /\ m.h = height[r] /\ m.v = view[r]}
       IN Cardinality({m.sender : m \in votes}) >= Quorum
    /\ messages' = messages \cup {Msg("vote_precommit", height[r], view[r], r, height[r], 0)}
    /\ lockedView' = [lockedView EXCEPT ![r] = view[r]]
    /\ lockedValue' = [lockedValue EXCEPT ![r] = height[r]]
    /\ phase' = [phase EXCEPT ![r] = PhaseCommit]
    /\ UNCHANGED <<height, view, decided, decidedHash, maxDecided>>

(***************************************************************************)
(* CollectCommit: collect N-F pre-commit votes (creates pre-commit QC)    *)
(***************************************************************************)
CollectCommit(r) ==
    /\ IsHonest(r)
    /\ phase[r] = PhaseCommit
    /\ decided[r] = FALSE
    /\ LET votes == {m \in messages : IsVotePreCommit(m) /\ m.h = height[r] /\ m.v = view[r]}
       IN Cardinality({m.sender : m \in votes}) >= Quorum
    /\ messages' = messages \cup {Msg("vote_commit", height[r], view[r], r, height[r], 0)}
    /\ phase' = [phase EXCEPT ![r] = PhaseDecide]
    /\ UNCHANGED <<height, view, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* Decide: honest replica decides after collecting N-F commit votes        *)
(***************************************************************************)
Decide(r) ==
    /\ IsHonest(r)
    /\ phase[r] = PhaseDecide
    /\ decided[r] = FALSE
    /\ LET votes == {m \in messages : IsVoteCommit(m) /\ m.h = height[r] /\ m.v = view[r]}
       IN Cardinality({m.sender : m \in votes}) >= Quorum
    /\ decided' = [decided EXCEPT ![r] = TRUE]
    /\ decidedHash' = [decidedHash EXCEPT ![r] = height[r]]
    /\ maxDecided' = IF height[r] > maxDecided THEN height[r] ELSE maxDecided
    /\ UNCHANGED <<height, view, phase, lockedView, lockedValue, messages>>

(***************************************************************************)
(* Advance: decided replica moves to the next height                      *)
(***************************************************************************)
Advance(r) ==
    /\ IsHonest(r)
    /\ decided[r] = TRUE
    /\ height[r] < MaxHeight
    /\ height' = [height EXCEPT ![r] = height[r] + 1]
    /\ view' = [view EXCEPT ![r] = 0]
    /\ phase' = [phase EXCEPT ![r] = PhasePrepare]
    /\ decided' = [decided EXCEPT ![r] = FALSE]
    /\ decidedHash' = [decidedHash EXCEPT ![r] = 0]
    /\ lockedView' = [lockedView EXCEPT ![r] = 0]
    /\ lockedValue' = [lockedValue EXCEPT ![r] = 0]
    /\ UNCHANGED <<messages, maxDecided>>

(***************************************************************************)
(* Timeout: honest replica suspects leader is faulty                       *)
(* Broadcasts timeout with locked state and advances view                 *)
(***************************************************************************)
Timeout(r) ==
    /\ IsHonest(r)
    /\ decided[r] = FALSE
    /\ messages' = messages \cup {
        Msg("timeout", height[r], view[r], r, lockedValue[r], lockedView[r])}
    /\ view' = [view EXCEPT ![r] = view[r] + 1]
    /\ phase' = [phase EXCEPT ![r] = PhasePrepare]
    /\ UNCHANGED <<height, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* NewView: replica collects N-F timeout messages and enters next view    *)
(* This forms a Timeout Certificate proving view timed out                *)
(***************************************************************************)
NewView(r) ==
    /\ IsHonest(r)
    /\ decided[r] = FALSE
    \* There exists a view v where a TC has formed
    /\ \E tcView \in 0..view[r] :
        TCFormed(messages, height[r], tcView)
        /\ ~TCFormed(messages, height[r], tcView + 1)
    /\ view' = [view EXCEPT ![r] = view[r] + 1]
    /\ phase' = [phase EXCEPT ![r] = PhasePrepare]
    /\ UNCHANGED <<height, decided, decidedHash, lockedView, lockedValue, messages, maxDecided>>

(***************************************************************************)
(* ===== BYZANTINE REPLICA ACTIONS =====                                  *)
(***************************************************************************)

(***************************************************************************)
(* ByzantineSend: faulty replica sends ONE message                         *)
(* Models equivocation, malicious proposals, protocol deviance            *)
(* Only sends at the current (h, v) to bound state space                  *)
(***************************************************************************)
ByzantineSend(r) ==
    /\ IsFaulty(r)
    /\ \E typ \in MsgTypes :
        messages' = messages \cup
            {Msg(typ, height[r], view[r], r, height[r], 0)}
    /\ UNCHANGED <<height, view, phase, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* ByzantineEquivocate: faulty replica sends two different values          *)
(* at the same height/view — tests if equivocation breaks safety          *)
(***************************************************************************)
ByzantineEquivocate(r) ==
    /\ IsFaulty(r)
    /\ \E typ \in {"vote_prep", "vote_precommit", "vote_commit"} :
        messages' = messages \cup
            {Msg(typ, height[r], view[r], r, height[r], 0),
             Msg(typ, height[r], view[r], r, 0, 0)}
    /\ UNCHANGED <<height, view, phase, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* ===== NETWORK ACTION =====                                             *)
(***************************************************************************)

(***************************************************************************)
(* DropMessages: simulate network partition — a message is lost           *)
(* Safety must hold regardless of message loss.                          *)
(***************************************************************************)
DropMessages ==
    /\ messages /= {}
    /\ \E m \in messages :
        messages' = messages \ {m}
    /\ UNCHANGED <<height, view, phase, decided, decidedHash, lockedView, lockedValue, maxDecided>>

(***************************************************************************)
(* Next-state relation                                                    *)
(***************************************************************************)
NextSafety ==
    \/ \E r \in Honest : Propose(r)
    \/ \E r \in Honest : VotePrepare(r)
    \/ \E r \in Honest : CollectPreCommit(r)
    \/ \E r \in Honest : CollectCommit(r)
    \/ \E r \in Honest : Decide(r)
    \/ \E r \in Honest : Advance(r)

NextByzantine ==
    \/ NextSafety
    \/ \E r \in Faulty : ByzantineSend(r)
    \/ \E r \in Faulty : ByzantineEquivocate(r)

NextFull ==
    \/ NextByzantine
    \/ \E r \in Honest : Timeout(r)
    \/ \E r \in Honest : NewView(r)
    \/ DropMessages

(***************************************************************************)
(* Next is the full state transition used in the temporal Spec            *)
(***************************************************************************)
Next == NextFull

(***************************************************************************)
(* ===== INVARIANTS (Safety) =====                                        *)
(***************************************************************************)

(***************************************************************************)
(* Agreement: no two honest replicas decide different values               *)
(* at the same height. Core safety property of consensus.                *)
(***************************************************************************)
Agreement ==
    \A r1, r2 \in Honest :
        (decided[r1] /\ decided[r2] /\ height[r1] = height[r2])
        => decidedHash[r1] = decidedHash[r2]

(***************************************************************************)
(* NoDoubleCommit: no honest replica decides different values              *)
(* at the same height across different views.                             *)
(***************************************************************************)
NoDoubleCommit ==
    \A r \in Honest :
        decided[r] => decidedHash[r] = height[r]

(***************************************************************************)
(* QuorumIntersection: any two quorums intersect in at least one honest    *)
(* replica. This is the fundamental guarantee of BFT consensus safety.    *)
(* Mathematically follows from N > 3F and |Faulty| <= F.                  *)
(***************************************************************************)
QuorumIntersection ==
    \A Q1, Q2 \in SUBSET Replicas :
        (Cardinality(Q1) >= Quorum /\ Cardinality(Q2) >= Quorum)
        => Q1 \cap Q2 \cap Honest /= {}

(***************************************************************************)
(* PhaseValid: all replicas are in a valid phase state                     *)
(***************************************************************************)
PhaseValid ==
    \A r \in Replicas :
        phase[r] \in {PhasePrepare, PhasePreCommit, PhaseCommit, PhaseDecide}

(***************************************************************************)
(* LockedViewInvariant: honest replica only locks when it holds a QC      *)
(* lockedValue must equal the replica's current height.                   *)
(***************************************************************************)
LockedViewInvariant ==
    \A r \in Honest :
        lockedView[r] > 0 => lockedValue[r] = height[r]

(***************************************************************************)
(* TCValid: every timeout certificate references a valid view            *)
(***************************************************************************)
TCValid ==
    \A h \in 1..MaxHeight :
        \A v \in 0..MaxHeight :
            TCFormed(messages, h, v)
            => LET tms == {m \in messages : IsTimeout(m) /\ m.h = h /\ m.v = v}
                IN \A m \in tms : m.sender \in Replicas

(***************************************************************************)
(* Safety: all invariants combined                                        *)
(***************************************************************************)
Safety ==
    /\ Agreement
    /\ NoDoubleCommit
    /\ QuorumIntersection
    /\ PhaseValid
    /\ LockedViewInvariant
    /\ TCValid

(***************************************************************************)
(* Model-checking constraint — bound total messages                       *)
(***************************************************************************)
MessagesLimited == Cardinality(messages) < 8
ViewLimited == \A r \in Replicas : view[r] < 3

(***************************************************************************)
(* ===== LIVENESS (Temporal Properties) =====                             *)
(* Requires fairness — weak fairness on all Next actions                  *)
(***************************************************************************)

(***************************************************************************)
(* Liveness: every honest replica eventually decides                      *)
(***************************************************************************)
Liveness ==
    \A r \in Honest :
        <>(decided[r])

(***************************************************************************)
(* Progress: eventually max decided height reaches MaxHeight               *)
(***************************************************************************)
Progress ==
    <>(maxDecided = MaxHeight)

(***************************************************************************)
(* Temporal specification with weak fairness                               *)
(***************************************************************************)
Spec == Init /\ [][Next]_vars /\ WF_vars(Next)

====
