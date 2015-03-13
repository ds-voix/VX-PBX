  The VX abbreviate "Very eXtendable". This language was researched to make VX-PBX configuration as easy as it ever possible. It produces properly linked PBX objects, written in the UPSERT configuration language.

  The syntax is inherited from well-known config files one (.conf,.cfg,.ini), as it seems to be suitable enough for block-schema definition. There was introduced 2 significant notions:
  * "Object" is declared in square brackets form of "OBJECT label [ext1 [[ext2]..[extN]]]"
 e.g. "[SCHEDULE WorkDays !]" to declare negated ("!") schedule.
  * "Action" is the special key name, that points the object implementation.
 e.g. "Action = schedule WorkDays" will switch the call processing to the action(s) defined under the "SCHEDULE" object labeled "WorkDays".
  Although, there are a number of implicit action names, such as "0".."9" for voice menus ("IVR") on-key-pressed actions ("MENU" object).

   !All labels are case sensitive! But the keywords are case insensitive.

   Object & Action with the same name are coupled together. Each couple is processed by pluggable module. All modules are resided in ./schema.d/action/ directory, in current design. If no module found to process the object|action, the exception will be generated and processing stops.

   Although, there are few more syntax extensions to make things more comfortable.
  * A number of methods to make remark:
   1. Triple-quotes (like python ones) at the beginning of the line surrounds the commented block. This block passes (properly commented out) to the UPSERT output.
   2. Single ";" or "#" at the beginning of the line is the simple comment. Such line is excluded from processing.
   3. Double ";" or "#" at the beginning of the line produces "# written remark" to the output.
   4. Single ";" or "#" at the end of the object definition "[OBJECT label] ; remark" passes the rest of the line to generated UPSERT object remark. The same behavior for the ";" or "#" at the end of the action definition "Action = SomeAction label ; remark".

   * "-//-" at the beginning of the not commented-out line breaks processing. Any text below means nothing.

   * "=" sign before action name triggers inline processing, if any is defined in relevant module.
 e.g. "Action == queue 9583253" produces rather long chain of PBX objects to enqueue the call the labeled phone number.

   The VX config is processed in one pass, line-by-line. Actions for the same objects are chained top-down.

   All the non-action keys sets respective labels (commonly, in lowercase to stay case insensitive for end-user). These keys have 2-level visibility: local to the current section (object) and global (sc. ROOT). The global key must be taken in case of local one is omitted and it means something to look at the ROOT.

   Below are placed a number of practical examples, explained line-by-line.
 [* (812)322-52-47 REDIRECT] ; "*" or "ROOT" for ROOT object
 ; (812)322-52-47 transforms to incoming 8123225247, so ant non-digits are eaten
 ; the PBX "BIND" will be set to "REDIRECT "
  Action == queue 9583253 ; The incoming 8123225247 will be enqueued to dial 9583253
 ; The LegA will be set to 3225247 or 8123225247, depending on local region settings.
 ; The proper Route must be present in PBX to make dial-out possible.

   The above was totally reduced form. Now, the complete syntax to do the same. Will be much suitable for UI and m.b. automation tasks.

[* 8123225247 REDIRECT]
 Action = queue 9583253 ; Now, 9583253 means the object label

[QUEUE 9583253] ; Queue object definition
 Action = dial 9583253 ; This is the label of DIAL objects

[DIAL 9583253] ; Dial object definition
 Dial = 9583253 ; Just dial this number, with all defaults

   And the same once again, with full qualified inline.
[* 8123225247 REDIRECT]
 Action = = queue = dial 9583253 P,R,'' 86400 | |
; Full inline form for dial is:
; dial <phone1>[,phone2[,..phoneN]] [P|R|S],[R|M],[A|B|] <Timeout s> | <OnBusy action> | <OnTimeout action>
; The hunting strategy may be one of (P[arallel]|S[erial]|R[oundRobin])
; The ringing behavior can be R[ing] for produce default PSTN call indication,
; or any other letters to play MOH. MOH key defines MOH class to use.

  Now, more practice with more objects.
[* 8123225247 REDIRECT]
"""
 Sample config
commented block
"""
Timeout = 15 ; Default timeout for dial etc.
CID = B ; A|B|<empty> [A|B] to set CID = Leg[A|B] when dial
Hunt = S ; P[arallel]|S[erial]|R[oundRobin] default 'P'
Hello = '4928' ; Play this file from /Sounds/Dir/REDIRECT/4928.(wav|alaw|<etc.>) before first queue.
MOH = SomeClass ; Use this MOH class by default
Indication = + ; Produce 3s "beep" on call entry. Or do indication for exact seconds number.
; There is although action "Indication" to to so at any point of call processing.
; For those who prefer PSTN slowness for any reason.
 Action == queue action redirect ; Enqueue to the ACTION object
; There is no successor for queue action, so new action must be defined to implement branching by SCHEDULE.

[Action redirect] ; This ACTION is needed because of SCHEDULE usage
 Action = schedule SchedName ; On-Schedule action (TRUE branch in block-schema)
 Action = menu 12 ; No-Schedule action (FALSE branch in block-schema)
; M.b. more actions in chain, but each must accept to define successor.

[SCHEDULE SchedName]
 A = 10:00-12:00 | * ; Any id may be used, excepting "Action"
 aa = 12:00-20:00^mon-fri^*^jan-dec^2015 ; Well-known Asterisk project notation
 ; Extended by YEAR at the end, and with free-form of delimiter (current PCRE is /[\s]*[|,;^][\s]*/)
 Action == queue 9583253,9211234567:86400 ; dial 9583253 for 15s, then 9211234567 for 1 day

"""
  Now, define n-level menu tree
"""
[MENU 12]
Hello = ; To be played once. "Prompt" will be repeated for "Repeat" times.
Prompt = hello ; Play /Sounds/Dir/menu/REDIRECT/hello
Timeout = 4 ; Wait for 4s after Prompt before TimeoutAction
Repeat = 1 ; Do TimeoutAction on 1 invalid input
 "0" = fax mail@domain.tld ; FAX machine to email "mail@domain.tld"
 "1" == queue 9583253
 "2" == queue 9583253 ; Queue already created, so inline is unnecessary. But stays correct too.
 "#" = menu xx
 TimeoutAction = queue 9583253 ; Queue already created, so no inline is really needed

[MENU xx]
 Parent = 12 ; Must be defined to make BACK action
 "1" == queue 9211234567
 ("2","3","4") = BACK ; The valid UPSERT syntax to set the same value for multiple fields
 ; 'BACK' is predefined to return to the parent menu, if one.
 "#" = ext 200a ; Switch to extension 200a

; Extensions may be defined like UPSERT ranges.
[EXT 200a..200c] ; Exten 'sip://REDIRECT+200[abc]@pbx.domain.tld'
 sip.secret = jMX=J7WWikxE ; sip password
 CallLimit = 1
 Timeout = 86400
; The OnBusy and OnTimeout actions
 TransferOnBusy = queue 9583253,9211234567:86400 ; At this point this is the label of already declared queue
 TransferOnTimeout = fax mail@domain.tld
