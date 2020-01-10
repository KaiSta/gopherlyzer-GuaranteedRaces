package test;

public class Test11 extends Thread {
  static int x;
  static int y;

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1
	x = 1;
	System.out.println("w(x)");
	y = 1;
	System.out.println("w(y)");
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			int tmp = y;
			System.out.println("r(y)");
			x = 2;
			System.out.println("w(x)");
		}
   }
   


   public static void main(String args[]) throws Exception {
	final Test11 thread1 = new Test11();
    final T2 thread2 = new T2();

	thread1.start();
	thread2.start();

	thread1.join();
	thread2.join();

   }
}
