//
 Set(LOCAL(fnc)=${CUT(fn,\(,1)});
 Set(LOCAL(arg)=${CUT(fn,\(,2-)});
 local args=${FIELDQTY(arg,\x29)}-1;
//NoOp(AAA=${FIELDQTY(arg,\x29)}=${CUT(arg,\),1-${args})}= =${arg}=);
// Set(LOCAL(arg)=${CUT(arg,\),1)});
 Set(LOCAL(arg)=${CUT(arg,\),1-${args})});


 switch ("${fnc}") {
  case "Func":
//   NoOp(HASH(res)=ODBC_FUNC(${arg},${cid},${BIND}));
   Set(HASH(res)=${ODBC_FUNC(${arg},${cid},${BIND})}); // Is function defined?
   if ("${HASH(res,Macro)}"!="") {
    &${HASH(res,Macro)}(${HASH(res,P1)},${HASH(res,P2)},${HASH(res,P3)},${HASH(res,P4)});
    NoOp(RESULT=${RESULT});
   }
   break;

  case "Exten":
   Set(HASH(res)=${ODBC_EXTEN(${arg},${BIND})});
   if ("${HASH(res,Exten)}"!="") {
    Set(LOCAL(args)=${REPLACE(arg,\|,\,)});
    Set(__dnid=${CUT(args,\,,1)});
    if ("${CUT(args,\,,2)}"!="") Set(__BIND=${CUT(args,\,,2)});
    &exten(fake);
   }
   break;

  default:
   Set(LOCAL(args)=${REPLACE(arg,\|,\,)});
   &${fnc}(${args});
 }

 NoOp(RESULT=${RESULT});
